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
