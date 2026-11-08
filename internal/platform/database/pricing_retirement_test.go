package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
	"testing"
	"time"
)

func TestLegacyPersonalFlagCannotAuthorizeIndividualPricing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := pricingTestAdmin(t, s)
	user := createTestUser(t, s, 49559)
	combo := saveTestCombo(t, s, "retired-personal", 1000, 30)
	now := time.Now().UTC()
	policy, err := s.PricingPolicy(ctx, "combo", combo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SavePricingPolicy(ctx, actor.ID, "fixed policy", policy, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE pricing_product_settings SET dynamic_enabled=1,personal_discounts_enabled=1 WHERE product_type='combo' AND product_id=$1`, combo.ID); err != nil {
		t.Fatal(err)
	}
	s.pricingCache.Clear()
	decision, err := s.PrepareCheckoutPricing(ctx, user.ID, "combo", combo.ID, combo.ID, nil, now)
	if err != nil || decision.DynamicEnabled || decision.PublicPrice.MinorInt64() != 1000 || decision.PersonalPrice != decision.PublicPrice {
		t.Fatal(decision, err)
	}
	var count int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_personal_decisions`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestHistoricalReservedRenewalDiscountStillSettlesExactly(t *testing.T) {
	s, user, actor, source, _, now := pricingRenewalFixture(t)
	ctx := context.Background()
	snapshot, _, err := s.prepareRenewalPriceSnapshot(ctx, source.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	// Seed the old authorized amount at publication, never by editing a quote.
	candidate := renewal.Candidate{Price: 1000, Eligible: true}
	quote, err := s.publishRenewalPrice(ctx, snapshot, candidate, []renewal.Candidate{candidate}, now)
	if err != nil {
		t.Fatal(err)
	}
	// These rows model an obligation already authorized by the retired release.
	if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_admin_overrides(id,scope_type,scope_id,command,reason,created_by,starts_at,expires_at,expected_version,previous_json,proposed_json,status,created_at) VALUES('legacy-campaign','user_product',$1,'PERSONAL_DISCOUNT','historical obligation',$2,$3,$4,1,'{}','{"budgetMinor":"1000"}','ACTIVE',$3)`, user.ID+":combo:"+source.ComboID, actor.ID, pricingStamp(now), pricingStamp(now.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_discount_spend(override_id,reference_id,user_id,discount_minor,created_at,status) VALUES('legacy-campaign',$1,$2,300,$3,'RESERVED')`, "renewal:"+quote.ID, user.ID, pricingStamp(now)); err != nil {
		t.Fatal(err)
	}
	input := RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 2, IdempotencyKey: "legacy-discount-obligation"}
	batch, err := s.Renew(ctx, input, now)
	if err != nil || batch.TotalPrice.MinorInt64() != 2000 {
		t.Fatal(batch, err)
	}
	replay, err := s.Renew(ctx, input, now)
	if err != nil || replay.ID != batch.ID {
		t.Fatal(replay, err)
	}
	var consumed int64
	if err = s.db.QueryRowContext(ctx, `SELECT SUM(discount_minor) FROM pricing_discount_spend WHERE override_id='legacy-campaign' AND status='CONSUMED'`).Scan(&consumed); err != nil || consumed != 600 {
		t.Fatal(consumed, err)
	}
}
