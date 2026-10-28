package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPricingAdmissionDistinguishesNoProxyGroupsFromMissingNodes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	combo := saveTestCombo(t, s, "no-proxy", 100, 30)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = pricingHealthAdmissionTx(ctx, tx, "combo", combo.ID, time.Now()); err != nil {
		t.Fatalf("combo without proxy groups: %v", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE combos SET included_squad_uuids='["missing-group"]' WHERE id=?`, combo.ID); err != nil {
		t.Fatal(err)
	}
	if err = pricingHealthAdmissionTx(ctx, tx, "combo", combo.ID, time.Now()); !errors.Is(err, pricing.CapacityUnsafe) {
		t.Fatalf("missing node should remain unsafe: %v", err)
	}
	if err = pricingHealthAdmissionTx(ctx, tx, "combo", "missing-combo", time.Now()); !errors.Is(err, pricing.CapacityUnsafe) {
		t.Fatalf("missing combo should remain unsafe: %v", err)
	}
}

func TestQueuedPricingDecisionsPreserveRenewalOnFailureAndDisableOnSuccess(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, 49771)
	c := saveTestCombo(t, s, "decision", 100, 30)
	now := time.Now().UTC()
	if _, err := s.AdjustBalance(ctx, u.ID, 100, "decision-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreatePurchase(ctx, PurchaseInput{UserID: u.ID, ComboID: c.ID, IdempotencyKey: "source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetAutoRenewal(ctx, u.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}
	input := PurchaseInput{UserID: u.ID, ComboID: c.ID, IdempotencyKey: "future"}
	q, err := s.QuotePurchase(ctx, input, now)
	if err != nil || !q.Queued || q.PricingDecision == nil || q.RenewalDecision == nil {
		t.Fatal(q, err)
	}
	if q.PricingDecision.Status != "UNCHANGED" || q.RenewalDecision.Status != "UNCHANGED" {
		t.Fatal("fixed decisions not terminal", q)
	}
	if _, err = s.CreatePurchase(ctx, input, now); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatal(err)
	}
	current, err := s.PurchaseByID(ctx, source.ID)
	if err != nil || !current.AutoRenewEnabled {
		t.Fatal("failed checkout disabled renewal", current, err)
	}
	var contracts int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_contracts WHERE purchase_id=?`, source.ID).Scan(&contracts); err != nil || contracts != 0 {
		t.Fatal("fixed decision froze legacy settlement", contracts, err)
	}
	if _, err = s.AdjustBalance(ctx, u.ID, 500, "decision-funding", "seed", now); err != nil {
		t.Fatal(err)
	}
	future, err := s.CreatePurchase(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	current, err = s.PurchaseByID(ctx, source.ID)
	if err != nil || current.AutoRenewEnabled {
		t.Fatal("queued checkout retained renewal", current, err)
	}
	replay, err := s.CreatePurchase(ctx, input, now.Add(24*time.Hour))
	if err != nil || replay.ID != future.ID {
		t.Fatal("replay charged twice", replay, err)
	}
	var debits int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE reference_id=? AND kind='purchase_debit'`, future.ID).Scan(&debits); err != nil || debits != 1 {
		t.Fatal(debits, err)
	}
}

func TestWalletDoesNotChangeDecisionButCouponScopeDoes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, 49772)
	c := saveTestCombo(t, s, "wallet-independent", 1000, 30)
	now := time.Now().UTC().Truncate(time.Hour).Add(time.Minute)
	coupon, err := s.SaveCoupon(ctx, coupons.CouponInput{Code: "DECISION50", Name: "Scoped", Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 5000, EligibleComboIDs: []string{c.ID}, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RedeemCoupon(ctx, u.ID, coupon.Code, "grant", now); err != nil {
		t.Fatal(err)
	}
	before, err := s.PreparePersonalPricing(ctx, u.ID, "combo", c.ID, c.ID, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AdjustBalance(ctx, u.ID, 90000, "decision-wallet", "funding", now); err != nil {
		t.Fatal(err)
	}
	after, err := s.PreparePersonalPricing(ctx, u.ID, "combo", c.ID, c.ID, nil, now.Add(time.Second))
	if err != nil || after.Version != before.Version || after.PersonalPrice != before.PersonalPrice {
		t.Fatal("wallet affected decision", before, after, err)
	}
	coupon.EligibleComboIDs = []string{"another-product"}
	if _, err = s.SaveCoupon(ctx, coupon.CouponInput, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	changed, err := s.PreparePersonalPricing(ctx, u.ID, "combo", c.ID, c.ID, nil, now.Add(3*time.Second))
	if err != nil || changed.Version == before.Version || changed.NetPrice.Minor != "1000" {
		t.Fatal("scope change retained coupon decision", changed, err)
	}
}

func TestFutureCheckoutRequiresBothCurrentAndSelectedDecision(t *testing.T) {
	s, user, _, source, policy, now := pricingRenewalFixture(t)
	ctx := context.Background()
	input := PurchaseInput{UserID: user.ID, ComboID: policy.ProductID, IdempotencyKey: "both-decisions"}
	q, err := s.QuotePurchase(ctx, input, now)
	if err != nil || q.RenewalDecision == nil || q.PricingDecision == nil {
		t.Fatal(q, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET state='CANCELLED' WHERE purchase_id=?`, source.ID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, _, _, err = s.quotePurchaseTx(ctx, tx, input, now); !errors.Is(err, ErrPricingDecisionPending) {
		tx.Rollback()
		t.Fatal("selected decision bypassed missing current decision", err)
	}
	tx.Rollback()
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM pricing_personal_decisions WHERE version=?`, q.PricingDecision.Version); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, _, _, err = s.quotePurchaseTx(ctx, tx, input, now); !errors.Is(err, ErrPricingDecisionPending) {
		tx.Rollback()
		t.Fatal("current decision bypassed missing selected decision", err)
	}
	tx.Rollback()
}
