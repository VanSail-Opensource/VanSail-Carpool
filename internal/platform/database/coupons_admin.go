package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

// SaveAdminCoupon binds an administrator edit to the definition that was merged,
// and commits the exact result together with an audit. Private reward definitions
// cannot be edited through the public coupon catalog.
func (s *Store) retryAttempt_SaveAdminCoupon(ctx context.Context, actorID string, input coupons.CouponInput, expectedUpdatedAt, now time.Time) (coupons.Coupon, error) {
	if input.ID == "" && !expectedUpdatedAt.IsZero() || input.ID != "" && expectedUpdatedAt.IsZero() {
		return coupons.Coupon{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coupons.Coupon{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return coupons.Coupon{}, err
	}
	var before *coupons.CouponInput
	previousCode := ""
	if input.ID != "" {
		previous, loadErr := scanCoupon(tx.QueryRowContext(ctx, couponSelect+` WHERE id=$1 AND admin_visible=1 FOR UPDATE`, input.ID))
		if loadErr != nil {
			return coupons.Coupon{}, loadErr
		}
		if !previous.UpdatedAt.Equal(expectedUpdatedAt) {
			return coupons.Coupon{}, ErrConflict
		}
		previousCode = previous.Code
		previous.Code = ""
		before = &previous.CouponInput
	}
	coupon, err := saveCouponTx(ctx, tx, input, now)
	if err != nil {
		return coupons.Coupon{}, err
	}
	// Codes authorize financial benefits. Preserve changed policy fields in the
	// audit without creating another copy of the redeemable credential.
	after := coupon.CouponInput
	after.Code = ""
	detail := map[string]any{"before": before, "after": after, "codeChanged": before != nil && previousCode != coupon.Code}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "coupon.save", "coupon", coupon.ID, detail, now.UTC()); err != nil {
		return coupons.Coupon{}, err
	}
	if err = tx.Commit(); err != nil {
		return coupons.Coupon{}, err
	}
	return coupon, nil
}
