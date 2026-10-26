package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

// Eligibility uses the same rules as checkout. Attached recurring renewal
// coupons retain their contractual applicability in the renewal path.
func pricingCouponsTx(ctx context.Context, tx *sql.Tx, user, combo string, squads []string, now time.Time) ([]coupons.Grant, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM coupon_grants WHERE user_id=? AND status='active' AND NOT EXISTS(SELECT 1 FROM coupon_grant_discards d WHERE d.grant_id=coupon_grants.id) ORDER BY id`, user)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	grants := []coupons.Grant{}
	for _, id := range ids {
		g, e := grantByIDTx(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if g.Coupon.Kind != coupons.KindPurchaseOnce && g.Coupon.Kind != coupons.KindPurchaseRecurring {
			continue
		}
		if e = couponAvailable(g.Coupon, now); e != nil {
			continue
		}
		if !g.Coupon.EligibleFor(combo, squads) {
			continue
		}
		if e = ensureCouponUseAvailableTx(ctx, tx, g.Coupon, user); e != nil {
			if errors.Is(e, ErrConflict) {
				continue
			}
			return nil, e
		}
		grants = append(grants, g)
	}
	return grants, nil
}

func pricingCouponDiscount(grants []coupons.Grant, price int64) (int64, error) {
	var best int64
	for _, g := range grants {
		value, err := coupons.CalculateDiscount(g.Coupon, price)
		if err != nil {
			return 0, err
		}
		best = max(best, value)
	}
	return best, nil
}
