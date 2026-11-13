package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestAdminCouponCannotChangeGrantedUseClassification(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 98_890)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	owner := createTestUser(t, store, 98_891)
	now := time.Now().UTC()
	combo := saveTestCombo(t, store, "coupon-kind-boundary", 100, 30)
	if _, err := store.AdjustBalance(ctx, owner.ID, 500, "coupon-policy-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	original, err := store.SaveAdminCoupon(ctx, actor.ID, coupons.CouponInput{
		Code: "BOUND_ONCE", Name: "Once", Kind: coupons.KindPurchaseOnce,
		DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 20, Active: true,
	}, time.Time{}, now)
	if err != nil {
		t.Fatal(err)
	}
	redeemed, err := store.RedeemCoupon(ctx, owner.ID, original.Code, "grant-once", now)
	if err != nil || redeemed.Grant == nil {
		t.Fatalf("grant = %+v, %v", redeemed, err)
	}
	for _, edit := range []coupons.CouponInput{
		{ID: original.ID, Code: original.Code, Name: "Changed kind", Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 20, Active: true},
		{ID: original.ID, Code: original.Code, Name: "Changed mode", Kind: coupons.KindPurchaseOnce, DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 20, Active: true},
	} {
		if _, err := store.SaveAdminCoupon(ctx, actor.ID, edit, original.UpdatedAt, now.Add(time.Second)); !errors.Is(err, coupons.ErrInvalidInput) {
			t.Fatalf("accepted a change to granted policy: %v", err)
		}
	}
	current, err := couponByID(ctx, store.DB(), original.ID)
	if err != nil || current.Kind != original.Kind || current.DiscountMode != original.DiscountMode ||
		current.Name != original.Name || !current.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("rejected edit changed definition: %+v, %v", current, err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='coupon.save' AND actor_user_id=$1 AND target_id=$2`, actor.ID, original.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("rejected edits left audits: %d, %v", audits, err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{
		UserID: owner.ID, ComboID: combo.ID, CouponGrantID: redeemed.Grant.ID, IdempotencyKey: "consume-once",
	}, now.Add(2*time.Second))
	if err != nil || purchase.RecurringDiscountAttached || purchase.PriceTXBMinor != 80 {
		t.Fatalf("stored grant changed use classification: %+v, %v", purchase, err)
	}
	if _, err := store.QuotePurchaseCoupon(ctx, coupons.PurchaseContext{
		UserID: owner.ID, GrantID: redeemed.Grant.ID, ComboID: combo.ID, GrossPriceMinor: 100,
	}, now.Add(3*time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("one-time grant became reusable: %v", err)
	}
	edit := original.CouponInput
	edit.Name, edit.ValueMinorOrBPS, edit.Active = "Updated future value", 30, false
	if _, err := store.SaveAdminCoupon(ctx, actor.ID, edit, original.UpdatedAt, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	var gross, discount, net int64
	if err := store.DB().QueryRowContext(ctx, `SELECT gross_price_minor,discount_minor,net_price_minor FROM coupon_uses WHERE purchase_id=$1`, purchase.ID).Scan(&gross, &discount, &net); err != nil || gross != 100 || discount != 20 || net != 80 {
		t.Fatalf("accepted future edit repriced historical use: %d/%d/%d, %v", gross, discount, net, err)
	}
	plan, err := store.AutoRenewalPlan(ctx, owner.ID, purchase.ID, now.Add(time.Minute))
	if err != nil || plan.DiscountMinor != 0 || plan.NetMinor != 100 {
		t.Fatalf("one-time use acquired a recurring attachment: %+v, %v", plan, err)
	}
}

func TestAdminCouponValueEditsPreserveAttachedRenewalPolicy(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 98_892)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	owner := createTestUser(t, store, 98_893)
	now := time.Now().UTC()
	combo := saveTestCombo(t, store, "coupon-value-boundary", 100, 30)
	if _, err := store.AdjustBalance(ctx, owner.ID, 500, "coupon-value-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	original, err := store.SaveAdminCoupon(ctx, actor.ID, coupons.CouponInput{
		Code: "BOUND_RECUR", Name: "Recurring", Kind: coupons.KindPurchaseRecurring,
		DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 2_000, Active: true,
	}, time.Time{}, now)
	if err != nil {
		t.Fatal(err)
	}
	redeemed, err := store.RedeemCoupon(ctx, owner.ID, original.Code, "grant-recurring", now)
	if err != nil || redeemed.Grant == nil {
		t.Fatalf("grant = %+v, %v", redeemed, err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{
		UserID: owner.ID, ComboID: combo.ID, CouponGrantID: redeemed.Grant.ID, IdempotencyKey: "attach-recurring",
	}, now.Add(time.Second))
	if err != nil || !purchase.RecurringDiscountAttached {
		t.Fatalf("recurring attachment = %+v, %v", purchase, err)
	}
	capMinor := int64(25)
	past := now.Add(-time.Hour)
	edit := original.CouponInput
	edit.ValueMinorOrBPS, edit.PercentCapMinor, edit.Active = 3_000, &capMinor, false
	edit.EligibleComboIDs, edit.ExpiresAt = []string{"future-combo"}, &past
	if _, err := store.SaveAdminCoupon(ctx, actor.ID, edit, original.UpdatedAt, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	plan, err := store.AutoRenewalPlan(ctx, owner.ID, purchase.ID, now.Add(time.Minute))
	if err != nil || plan.DiscountMinor != 25 || plan.NetMinor != 75 {
		t.Fatalf("attached renewal lost the current value/cap policy: %+v, %v", plan, err)
	}
	var discount, net int64
	if err := store.DB().QueryRowContext(ctx, `SELECT discount_minor,net_price_minor FROM coupon_uses WHERE purchase_id=$1`, purchase.ID).Scan(&discount, &net); err != nil || discount != 20 || net != 80 {
		t.Fatalf("attached policy edit changed settled evidence: %d/%d, %v", discount, net, err)
	}
}
