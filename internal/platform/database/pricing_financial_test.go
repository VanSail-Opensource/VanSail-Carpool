package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func pricingRenewalFixture(t *testing.T) (*Store, model.User, model.User, model.Purchase, pricing.Policy, time.Time) {
	t.Helper()
	ctx := context.Background()
	s, parent, _ := xnodeTrafficFixture(t)
	user, err := s.UserByID(ctx, parent.UserID)
	if err != nil {
		t.Fatal(err)
	}
	actor := pricingTestAdmin(t, s)
	group := saveTestSquad(t, s, "pricing-node", 0, true)
	if err = s.SaveXNodeGroup(ctx, xnode.Group{ID: group.GroupID, Name: "Pricing", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: "node", InboundTag: "in"}}}); err != nil {
		t.Fatal(err)
	}
	addon := saveTestSquad(t, s, "pricing-addon", 300, true)
	combo := saveTestCombo(t, s, "Pricing", 1000, 30, group.ID)
	now := time.Now().UTC()
	if _, err = s.AdjustBalance(ctx, user.ID, 10000, "pricing-funding", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "pricing-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PricingPolicy(ctx, "combo", combo.ID)
	if err != nil {
		t.Fatal(err)
	}
	p.DynamicEnabled, p.RenewalMode, p.PersonalDiscounts = true, "DYNAMIC", true
	p.MaxIncreaseBPS, p.MaxDiscountBPS = 9000, 9000 // Caller input must not set calculated bounds.
	p, err = s.SavePricingPolicy(ctx, actor.ID, "enable future renewals", p, now)
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxIncreaseBPS != 0 || p.MaxDiscountBPS != 0 || p.PurchaseMode != "FIXED" {
		t.Fatal("manual limits or purchase activation leaked", p)
	}
	s.pricingHealth.Observe(pricing.Observation{NodeID: "node", At: now, ReportedAt: now, Connected: true, Ready: true, Synchronized: true, Complete: true, CPU: 20, Memory: 20})
	return s, user, actor, source, p, now
}

func TestPricingAutomaticRenewalPolicyAndBudgetedBatch(t *testing.T) {
	s, user, actor, source, p, now := pricingRenewalFixture(t)
	ctx := context.Background()
	// An ordinary administrator can immediately apply a material discount.
	grant, err := s.CreatePricingIntervention(ctx, actor.ID, PricingIntervention{ScopeType: "user_product", ScopeID: user.ID + ":combo:" + p.ProductID, Command: "PERSONAL_DISCOUNT", DiscountBPS: 3000, Budget: 1000, Reason: "retention campaign", ExpiresAt: now.Add(time.Hour)}, now)
	if err != nil || grant.Status != "ACTIVE" {
		t.Fatal(grant, err)
	}
	q, err := s.PreparePricingRenewal(ctx, source.ID, now)
	if err != nil || q.State != "AUTHORIZED" || q.Price != 1000 {
		t.Fatal("approval-free preparation", q, err)
	}
	input := RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 2, IdempotencyKey: "dynamic-batch"}
	batch, err := s.Renew(ctx, input, now)
	if err != nil || batch.TotalPrice.MinorInt64() != 2000 || len(batch.Purchases) != 2 {
		t.Fatal(batch, err)
	}
	for _, term := range batch.Purchases {
		c, e := s.PricingContract(ctx, user.ID, term.ID)
		if e != nil || c == nil || c.Price != 1300 {
			t.Fatal("campaign became perpetual", c, e)
		}
		assertRenewalAddonCharge(t, s, term.ID, 300)
	}
	replayed, err := s.Renew(ctx, input, now)
	if err != nil || replayed.ID != batch.ID {
		t.Fatal("idempotent replay changed", replayed, err)
	}
	var spent int64
	if err = s.db.QueryRowContext(ctx, `SELECT SUM(discount_minor) FROM pricing_discount_spend WHERE status='CONSUMED'`).Scan(&spent); err != nil || spent != 600 {
		t.Fatal(spent, err)
	}
	original, err := s.PurchaseByID(ctx, source.ID)
	if err != nil || original.PriceTXBMinor != 1300 {
		t.Fatal("historical charge changed", original, err)
	}
	var notices int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='pricing_change_notice'`).Scan(&notices); err != nil || notices < 2 {
		t.Fatal("missing notification", notices, err)
	}
}

func TestPricingHealthDefersWithoutDebitAndRecovers(t *testing.T) {
	s, user, _, source, _, now := pricingRenewalFixture(t)
	ctx := context.Background()
	q, err := s.PreparePricingRenewal(ctx, source.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	input := RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 1, IdempotencyKey: "health-renewal"}
	before, err := s.Balance(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.pricingHealth.Observe(pricing.Observation{NodeID: "node", At: now.Add(time.Minute), ReportedAt: now, Stale: true})
	if _, err = s.Renew(ctx, input, now.Add(time.Minute)); !errors.Is(err, pricing.RenewalHealthDeferred) {
		t.Fatal(err)
	}
	after, err := s.Balance(ctx, user.ID)
	if err != nil || after != before {
		t.Fatal("failed fulfillment debited wallet", before, after, err)
	}
	for i := 0; i <= 16; i++ {
		at := now.Add(2*time.Minute + time.Duration(i)*20*time.Second)
		s.pricingHealth.Observe(pricing.Observation{NodeID: "node", At: at, ReportedAt: at, Connected: true, Ready: true, Synchronized: true, Complete: true, CPU: 20, Memory: 20})
	}
	at := now.Add(7*time.Minute + 20*time.Second)
	batch, err := s.Renew(ctx, input, at)
	if err != nil || batch.TotalPrice.MinorInt64() != q.Price {
		t.Fatal("recovery changed immutable amount", batch, q, err)
	}
}
