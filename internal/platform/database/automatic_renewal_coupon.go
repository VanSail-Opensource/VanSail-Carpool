package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

// quoteAttachedRecurringDiscountTx uses only the attached grant's current
// discount mode, value, and cap. It intentionally ignores later availability,
// quota, eligibility, kind, and wallet-discard changes.
func quoteAttachedRecurringDiscountTx(ctx context.Context, tx *sql.Tx, userID, grantID string, grossMinor int64) (coupons.Discount, error) {
	base := coupons.Discount{GrossMinor: grossMinor, NetMinor: grossMinor}
	grant, err := grantByIDTx(ctx, tx, grantID)
	if err != nil {
		return coupons.Discount{}, err
	}
	if grant.UserID != userID {
		return coupons.Discount{}, ErrConflict
	}
	value, err := attachedDiscountMinor(grant.Coupon, grossMinor)
	if err != nil {
		return coupons.Discount{}, err
	}
	if value == 0 {
		return base, nil
	}
	return coupons.Discount{GrantID: grant.ID, CouponID: grant.Coupon.ID, CouponCode: grant.Coupon.Code,
		GrossMinor: grossMinor, DiscountMinor: value, NetMinor: grossMinor - value, Recurring: true}, nil
}

func attachedDiscountMinor(coupon coupons.Coupon, grossMinor int64) (int64, error) {
	if coupon.DiscountMode != coupons.DiscountFixed && coupon.DiscountMode != coupons.DiscountPercent {
		return 0, nil
	}
	if coupon.ValueMinorOrBPS <= 0 {
		return 0, nil
	}
	current := coupons.Coupon{CouponInput: coupons.CouponInput{
		Kind: coupons.KindPurchaseRecurring, DiscountMode: coupon.DiscountMode,
		ValueMinorOrBPS: coupon.ValueMinorOrBPS, PercentCapMinor: coupon.PercentCapMinor,
	}}
	return coupons.CalculateDiscount(current, grossMinor)
}
