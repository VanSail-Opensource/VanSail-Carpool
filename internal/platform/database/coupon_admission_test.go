package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func couponAdmissionInput(kind coupons.CouponKind) coupons.CouponInput {
	input := coupons.CouponInput{Code: "ADMISSION", Name: "Admission", Kind: kind, ValueMinorOrBPS: 50, Active: true}
	if kind == coupons.KindBalanceMultiply {
		input.ValueMinorOrBPS = 15_000
	} else if kind == coupons.KindPurchaseOnce || kind == coupons.KindPurchaseRecurring {
		input.DiscountMode = coupons.DiscountFixed
	}
	return input
}

func TestCouponRedemptionRequiresCurrentOwnerButPreservesReplay(t *testing.T) {
	t.Parallel()
	for _, kind := range []coupons.CouponKind{coupons.KindBalanceAdd, coupons.KindBalanceMultiply, coupons.KindPurchaseOnce, coupons.KindPurchaseRecurring} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			owner := createTestUser(t, store, 32_601)
			now := time.Now().UTC()
			if _, err := store.AdjustBalance(ctx, owner.ID, 1_000, "coupon-admission-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			coupon, err := store.SaveCoupon(ctx, couponAdmissionInput(kind), now)
			if err != nil {
				t.Fatal(err)
			}
			first, err := store.RedeemCoupon(ctx, owner.ID, coupon.Code, "accepted", now)
			if err != nil {
				t.Fatal(err)
			}
			facts := func() [4]int64 {
				var result [4]int64
				if err := store.DB().QueryRowContext(ctx, `SELECT
					(SELECT txb_minor FROM balances WHERE user_id=$1),
					(SELECT COUNT(*) FROM coupon_redemptions WHERE user_id=$1),
					(SELECT COUNT(*) FROM coupon_grants WHERE user_id=$1),
					(SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1)`, owner.ID).Scan(&result[0], &result[1], &result[2], &result[3]); err != nil {
					t.Fatal(err)
				}
				return result
			}
			before := facts()
			for _, status := range []string{"purging", "purged"} {
				if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status=$1 WHERE id=$2`, status, owner.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := store.RedeemCoupon(ctx, owner.ID, coupon.Code, "fresh-"+status, now.Add(time.Second)); !errors.Is(err, ErrAccountUnavailable) {
					t.Fatal("inactive owner received a fresh benefit", status, err)
				}
				replay, err := store.RedeemCoupon(ctx, owner.ID, coupon.Code, "accepted", now.Add(time.Second))
				if err != nil || !replay.Replayed || replay.ID != first.ID || replay.BalanceAfterMinor != first.BalanceAfterMinor {
					t.Fatal("accepted receipt lost after lifecycle change", status, replay, err)
				}
				if after := facts(); after != before {
					t.Fatal("refusal or replay changed financial evidence", before, after)
				}
			}
			// A Telegram delivery flag alone does not retire the financial owner.
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='active',bot_blocked=1 WHERE id=$1`, owner.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.RedeemCoupon(ctx, owner.ID, coupon.Code, "fresh-purging", now.Add(2*time.Second)); err != nil {
				t.Fatal("refused request could not be retried after recovery", err)
			}
		})
	}
}

func TestCouponResponseFailureRollsBackFinancialMutation(t *testing.T) {
	t.Parallel()
	for _, kind := range []coupons.CouponKind{coupons.KindBalanceAdd, coupons.KindPurchaseRecurring} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			owner := createTestUser(t, store, 32_602)
			now := time.Now().UTC()
			if _, err := store.AdjustBalance(ctx, owner.ID, 1_000, "coupon-response-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			coupon, err := store.SaveCoupon(ctx, couponAdmissionInput(kind), now)
			if err != nil {
				t.Fatal(err)
			}
			// Inject a stored result that cannot be decoded, after financial work
			// has occurred but before its transaction may commit.
			if _, err := store.DB().ExecContext(ctx, `CREATE FUNCTION corrupt_coupon_response() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN NEW.created_at='invalid-fixture-timestamp'; RETURN NEW; END $$`); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE TRIGGER corrupt_coupon_response BEFORE INSERT ON coupon_redemptions FOR EACH ROW EXECUTE FUNCTION corrupt_coupon_response()`); err != nil {
				t.Fatal(err)
			}
			if _, err := store.RedeemCoupon(ctx, owner.ID, coupon.Code, "response", now); err == nil {
				t.Fatal("invalid retained response was accepted")
			}
			var balance, redemptions, grants, entries int64
			if err := store.DB().QueryRowContext(ctx, `SELECT
				(SELECT txb_minor FROM balances WHERE user_id=$1),
				(SELECT COUNT(*) FROM coupon_redemptions WHERE user_id=$1),
				(SELECT COUNT(*) FROM coupon_grants WHERE user_id=$1),
				(SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind LIKE 'coupon_%')`, owner.ID).Scan(&balance, &redemptions, &grants, &entries); err != nil {
				t.Fatal(err)
			}
			if balance != 1_000 || redemptions != 0 || grants != 0 || entries != 0 {
				t.Fatal("failed response committed financial effects", balance, redemptions, grants, entries)
			}
			if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER corrupt_coupon_response ON coupon_redemptions`); err != nil {
				t.Fatal(err)
			}
			result, err := store.RedeemCoupon(ctx, owner.ID, coupon.Code, "response", now)
			if err != nil || result.Replayed || result.Coupon.ID != coupon.ID || !result.CreatedAt.Equal(now) {
				t.Fatal("valid retry did not return its committed receipt", result, err)
			}
			if kind == coupons.KindBalanceAdd && result.BalanceDeltaMinor != 50 || kind == coupons.KindPurchaseRecurring && result.Grant == nil {
				t.Fatal("committed benefit missing", result)
			}
		})
	}
}
