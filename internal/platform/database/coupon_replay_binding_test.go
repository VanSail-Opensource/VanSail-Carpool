package database

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestCouponRedemptionReplayBindsOriginalCodeAfterRename(t *testing.T) {
	t.Parallel()
	for _, kind := range []coupons.CouponKind{coupons.KindBalanceAdd, coupons.KindBalanceMultiply, coupons.KindPurchaseOnce, coupons.KindPurchaseRecurring} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := newTestStore(t)
			user := createTestUser(t, store, 32_250)
			now := time.Now().UTC()
			if _, err := store.AdjustBalance(ctx, user.ID, 1_000, "replay-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			input := coupons.CouponInput{Code: "ORIGINAL", Name: "Replay binding", Kind: kind, ValueMinorOrBPS: 50, Active: true}
			if kind == coupons.KindBalanceMultiply {
				input.ValueMinorOrBPS = 15_000
			} else if kind == coupons.KindPurchaseOnce || kind == coupons.KindPurchaseRecurring {
				input.DiscountMode = coupons.DiscountFixed
			}
			coupon, err := store.SaveCoupon(ctx, input, now)
			if err != nil {
				t.Fatal(err)
			}
			first, err := store.RedeemCoupon(ctx, user.ID, "original", "same-request", now)
			if err != nil {
				t.Fatal(err)
			}
			input.ID, input.Code = coupon.ID, "RENAMED"
			if _, err := store.SaveCoupon(ctx, input, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			replay, err := store.RedeemCoupon(ctx, user.ID, "ORIGINAL", "same-request", now.Add(2*time.Second))
			if err != nil || !replay.Replayed || replay.ID != first.ID {
				t.Fatalf("original request lost after definition rename: %+v, %v", replay, err)
			}
			for _, changedCode := range []string{"RENAMED", "DIFFERENT"} {
				if _, err := store.RedeemCoupon(ctx, user.ID, changedCode, "same-request", now.Add(3*time.Second)); !errors.Is(err, ErrConflict) {
					t.Fatalf("changed code %q reused an existing request: %v", changedCode, err)
				}
			}
			balance, err := store.Balance(ctx, user.ID)
			if err != nil || balance.Minor != strconv.FormatInt(first.BalanceAfterMinor, 10) {
				t.Fatalf("replays changed balance: %+v, %v", balance, err)
			}
			var redemptions int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_redemptions WHERE user_id=$1`, user.ID).Scan(&redemptions); err != nil || redemptions != 1 {
				t.Fatalf("replay wrote additional redemption: %d, %v", redemptions, err)
			}
		})
	}
}
