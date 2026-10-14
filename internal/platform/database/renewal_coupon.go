package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

// quoteRecurringRenewalCouponTx applies the recurring attachment recorded on
// the source purchase without recording a coupon use. The purchase flag is
// authoritative: it excludes one-time grants even if their mutable definition
// is later edited, while preserving recurring discounts after edits, expiry,
// quota exhaustion, or wallet discard.
func quoteRecurringRenewalCouponTx(ctx context.Context, tx *sql.Tx, userID, grantID string, grossMinor int64) (coupons.Discount, error) {
	return quoteAttachedRecurringDiscountTx(ctx, tx, userID, grantID, grossMinor)
}
