package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestPricingCouponInputsUseExactExpiry(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 32_207)
	now := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	var liveID string
	for _, input := range []struct {
		code    string
		expires time.Time
	}{
		{"PRICING_WHOLE", now},
		{"PRICING_NANO", now.Add(900 * time.Nanosecond)},
	} {
		coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: input.code, Name: input.code,
			Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountFixed,
			ValueMinorOrBPS: 20, ExpiresAt: &input.expires, Active: true}, now.Add(-time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		grant, err := store.GrantCoupon(ctx, user.ID, coupon.ID, "test", input.code, now.Add(-time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if input.expires.After(now) {
			liveID = grant.ID
		}
	}
	prepared, err := store.pricingReadContext(ctx, user.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name string
		ctx  context.Context
	}{{"prepared", prepared}, {"fresh", pricingFreshContext(prepared)}} {
		tx, err := store.DB().BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		grants, pricingErr := pricingCouponsTx(input.ctx, tx, user.ID, "test", nil, now)
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if pricingErr != nil || len(grants) != 1 || grants[0].ID != liveID {
			t.Fatalf("%s pricing grants = %+v, %v; want live %s", input.name, grants, pricingErr, liveID)
		}
	}
}

func TestPricingAddonCouponScopeExcludesExpiredCurrentPurchase(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	user := createTestUser(t, s, 32_208)
	now := time.Now().UTC()
	combo := saveTestCombo(t, s, "Coupon source", 100, 30)
	addon := saveTestSquad(t, s, "Scoped add-on", 300, true)
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, "scope-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "scope-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	coupon, err := s.SaveCoupon(ctx, coupons.CouponInput{Code: "CURRENT_SCOPE", Name: "Owned combo",
		Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountFixed,
		ValueMinorOrBPS: 20, EligibleComboIDs: []string{combo.ID}, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GrantCoupon(ctx, user.ID, coupon.ID, "test", "scope-grant", now); err != nil {
		t.Fatal(err)
	}
	at := now.Add(time.Hour).Truncate(time.Second)
	for _, live := range []bool{false, true} {
		until := at
		if live {
			until = until.Add(time.Nanosecond)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(until), source.ID); err != nil {
			t.Fatal(err)
		}
		grants, err := s.offerCouponInputs(ctx, user.ID, "addon", addon.GroupID, at)
		if err != nil || (len(grants) == 1) != live || len(grants) > 1 {
			t.Fatal("expired purchase lent its combo scope to an add-on offer", live, grants, err)
		}
	}
}
