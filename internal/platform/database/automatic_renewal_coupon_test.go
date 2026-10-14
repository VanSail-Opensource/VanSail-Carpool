package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestAutomaticRenewalAttachedCouponUsesLatestDiscountOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_004)
	combo := saveTestCombo(t, store, "automatic-coupon", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "coupon-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	limit := int64(1)
	coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "AUTORECUR", Name: "Automatic", Kind: coupons.KindPurchaseRecurring,
		DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 20, GlobalUseLimit: &limit, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	redeemed, err := store.RedeemCoupon(ctx, user.ID, coupon.Code, "automatic-coupon-redeem", now)
	if err != nil || redeemed.Grant == nil {
		t.Fatalf("RedeemCoupon() = (%+v, %v)", redeemed, err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, CouponGrantID: redeemed.Grant.ID, IdempotencyKey: "automatic-coupon-source"}, now)
	if err != nil || !source.RecurringDiscountAttached {
		t.Fatalf("CreatePurchase() = (%+v, %v), want recurring attachment", source, err)
	}
	if err := store.DiscardCouponGrant(ctx, user.ID, redeemed.Grant.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	past := stamp(now.Add(-time.Hour))
	if _, err := store.DB().ExecContext(ctx, `UPDATE coupon_definitions SET active=0,kind='balance_add',discount_mode='fixed',value_minor_or_bps=30,expires_at=? WHERE id=?`, past, coupon.ID); err != nil {
		t.Fatal(err)
	}
	plan, err := store.AutoRenewalPlan(ctx, user.ID, source.ID, now.Add(time.Hour))
	if err != nil || plan.DiscountMinor != 30 || plan.NetMinor != 70 {
		t.Fatalf("AutoRenewalPlan() = (%+v, %v), want latest attached 30 discount", plan, err)
	}
}

func TestAutomaticRenewalExcludesOneTimeCoupon(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_005)
	combo := saveTestCombo(t, store, "automatic-once", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "coupon-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "AUTOONCE", Name: "One time", Kind: coupons.KindPurchaseOnce,
		DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 20, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	redeemed, err := store.RedeemCoupon(ctx, user.ID, coupon.Code, "automatic-once-redeem", now)
	if err != nil || redeemed.Grant == nil {
		t.Fatalf("RedeemCoupon() = (%+v, %v)", redeemed, err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, CouponGrantID: redeemed.Grant.ID, IdempotencyKey: "automatic-once-source"}, now)
	if err != nil {
		t.Fatalf("CreatePurchase() = (%+v, %v)", source, err)
	}
	if source.RecurringDiscountAttached {
		t.Fatal("one-time purchase marked as recurring discount attachment")
	}
	plan, err := store.AutoRenewalPlan(ctx, user.ID, source.ID, now.Add(time.Hour))
	if err != nil || plan.DiscountMinor != 0 || plan.NetMinor != 100 {
		t.Fatalf("AutoRenewalPlan() = (%+v, %v), want no one-time discount", plan, err)
	}
}

func TestLegacyRenewalUsesPersistedRecurringAttachment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_006)
	combo := saveTestCombo(t, store, "legacy-recurring", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "coupon-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "LEGACYRECUR", Name: "Recurring", Kind: coupons.KindPurchaseRecurring,
		DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 20, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	redeemed, err := store.RedeemCoupon(ctx, user.ID, coupon.Code, "legacy-recurring-redeem", now)
	if err != nil || redeemed.Grant == nil {
		t.Fatalf("RedeemCoupon() = (%+v, %v)", redeemed, err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, CouponGrantID: redeemed.Grant.ID, IdempotencyKey: "legacy-recurring-source"}, now)
	if err != nil || !source.RecurringDiscountAttached {
		t.Fatalf("CreatePurchase() = (%+v, %v), want recurring attachment", source, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE coupon_definitions SET active=0,kind='purchase_once',value_minor_or_bps=30 WHERE id=?`, coupon.ID); err != nil {
		t.Fatal(err)
	}
	quote, err := store.RenewalQuote(ctx, user.ID, source.ID, 1, now.Add(time.Hour))
	if err != nil || quote.Discount.Minor != "30" || quote.PricePerTerm.Minor != "70" || quote.CouponGrantID == nil {
		t.Fatalf("RenewalQuote() = (%+v, %v), want persisted recurring discount", quote, err)
	}
}
