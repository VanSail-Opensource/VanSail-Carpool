package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestStoredCouponNumericShapeFailsBeforeCheckoutOrRenewalDebit(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		value      int64
		cap        any
	}{
		{"over one hundred percent", "percent", 10001, nil},
		{"fixed percentage cap", "fixed", 20, int64(10)},
		{"missing discount mode", "", 20, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Now().UTC()
			user := createTestUser(t, store, 950023)
			combo := saveTestCombo(t, store, "corrupt coupon", 100, 1)
			if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "STORED", Name: "Stored discount", Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 20, Active: true}, now)
			if err != nil {
				t.Fatal(err)
			}
			redemption, err := store.RedeemCoupon(ctx, user.ID, coupon.Code, "redeem", now)
			if err != nil || redemption.Grant == nil {
				t.Fatal(redemption, err)
			}
			source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, CouponGrantID: redemption.Grant.ID, IdempotencyKey: "source"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE coupon_definitions SET discount_mode=$1,value_minor_or_bps=$2,percent_cap_minor=$3 WHERE id=$4`, tc.mode, tc.value, tc.cap, coupon.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = store.QuotePurchaseCoupon(ctx, coupons.PurchaseContext{UserID: user.ID, GrantID: redemption.Grant.ID, ComboID: combo.ID, GrossPriceMinor: 100}, now); !errors.Is(err, coupons.ErrInvalidInput) {
				t.Fatal("malformed checkout discount accepted", err)
			}
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, quoteErr := quoteAttachedRecurringDiscountTx(ctx, tx, user.ID, redemption.Grant.ID, 100)
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(quoteErr, coupons.ErrInvalidInput) {
				t.Fatal("malformed recurring discount accepted", quoteErr)
			}
			if _, err = store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, CouponGrantID: redemption.Grant.ID, IdempotencyKey: "rejected"}, now); err == nil {
				t.Fatal("malformed checkout committed")
			}
			if _, err = store.Renew(ctx, RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 1, IdempotencyKey: "rejected-renewal"}, now); err == nil {
				t.Fatal("malformed renewal committed")
			}
			if balance := adminWorkflowBalance(t, store, user.ID); balance != 920 {
				t.Fatal("malformed discount changed wallet", balance)
			}
			assertRowCount(t, store, "purchases", 1)
			assertRowCount(t, store, "coupon_uses", 1)
		})
	}
}
