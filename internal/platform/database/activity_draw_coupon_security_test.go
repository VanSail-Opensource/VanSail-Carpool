package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestDrawCouponCannotBeRedeemedByCode(t *testing.T) {
	for _, kind := range []activity.RewardKind{activity.RewardCouponOnce, activity.RewardCouponRecurring} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := newTestStore(t)
			now := time.Now().UTC()
			winner := createTestUser(t, store, 31801)
			other := createTestUser(t, store, 31802)
			if _, err := store.AdjustBalance(ctx, winner.ID, 1000, "seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{
				Name: "Private coupon", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
				Prizes: []activity.PrizeInput{{Name: "Discount", ProbabilityBPS: 10000, Reward: activity.Reward{
					Kind: kind, DiscountMode: "fixed", Range: &activity.ValueRange{Min: 50, Max: 50, Distribution: "uniform"},
				}}},
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.PlayLuckyDraw(ctx, winner.ID, draw.ID, "win", fixedActivityRandom{}, now); err != nil {
				t.Fatal(err)
			}
			grants, err := store.ListCouponGrants(ctx, winner.ID, now)
			if err != nil || len(grants) != 1 {
				t.Fatalf("winner grants = %v, %v", grants, err)
			}
			for _, userID := range []string{winner.ID, other.ID} {
				if _, err = store.RedeemCoupon(ctx, userID, grants[0].Coupon.Code, "copy", now); !errors.Is(err, ErrNotFound) {
					t.Fatalf("redeem private coupon for %s: %v", userID, err)
				}
			}
			combo := saveTestCombo(t, store, "Coupon purchase", 100, 30)
			discount, err := store.QuotePurchaseCoupon(ctx, coupons.PurchaseContext{
				UserID: winner.ID, GrantID: grants[0].ID, ComboID: combo.ID, GrossPriceMinor: 100,
			}, now)
			if err != nil || discount.NetMinor != 50 {
				t.Fatalf("legitimate grant quote = %+v, %v", discount, err)
			}
			// Simulate a code copy minted before the security fix.
			copyGrant, err := store.GrantCoupon(ctx, other.ID, grants[0].Coupon.ID, "code", grants[0].Coupon.Code, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.QuotePurchaseCoupon(ctx, coupons.PurchaseContext{
				UserID: other.ID, GrantID: copyGrant.ID, ComboID: combo.ID, GrossPriceMinor: 100,
			}, now); !errors.Is(err, ErrNotFound) {
				t.Fatalf("historical code copy quote: %v", err)
			}
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, quoteErr := quoteAttachedRecurringDiscountTx(ctx, tx, other.ID, copyGrant.ID, 100)
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(quoteErr, ErrNotFound) {
				t.Fatalf("historical code copy renewal: %v", quoteErr)
			}
			wallet, err := store.ListCouponGrants(ctx, other.ID, now)
			if err != nil || len(wallet) != 0 {
				t.Fatalf("copied grants in wallet = %v, %v", wallet, err)
			}
			public, err := store.SaveCoupon(ctx, coupons.CouponInput{
				Code: "PUBLIC_DRAW_AUDIT", Name: "Public coupon", Kind: coupons.KindPurchaseOnce,
				DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 10, Active: true,
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.RedeemCoupon(ctx, other.ID, public.Code, "public", now); err != nil {
				t.Fatalf("public coupon redemption: %v", err)
			}
		})
	}
}
