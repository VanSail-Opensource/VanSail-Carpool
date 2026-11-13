package database

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"time"
)

const couponSelect = `SELECT id,code,name,kind,discount_mode,value_minor_or_bps,percent_cap_minor,
	eligible_combo_ids,eligible_squad_ids,expires_at,global_use_limit,per_user_use_limit,active,created_at,updated_at,
	(SELECT COUNT(*) FROM coupon_uses WHERE coupon_id=coupon_definitions.id) +
	(SELECT COUNT(*) FROM coupon_redemptions WHERE coupon_id=coupon_definitions.id AND coupon_definitions.kind IN ('balance_add','balance_multiply'))
	FROM coupon_definitions`

// SaveCoupon is the raw definition writer used by internal fixtures. Administrator
// requests must use SaveAdminCoupon to require a revision and atomic actor audit.
func (s *Store) retryAttempt_SaveCoupon(ctx context.Context, input coupons.CouponInput, now time.Time) (coupons.Coupon, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coupons.Coupon{}, err
	}
	defer tx.Rollback()
	coupon, err := saveCouponTx(ctx, tx, input, now)
	if err != nil {
		return coupons.Coupon{}, err
	}
	if err = tx.Commit(); err != nil {
		return coupons.Coupon{}, err
	}
	return coupon, nil
}

// ListCoupons returns coupon definitions newest first.

func (s *Store) ListCoupons(ctx context.Context, activeOnly bool) ([]coupons.Coupon, error) {
	query := couponSelect
	now := time.Now().UTC()
	if activeOnly {
		query += ` WHERE admin_visible=1 AND active=1`
	} else {
		query += ` WHERE admin_visible=1`
	}
	query += ` ORDER BY created_at DESC,id DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list coupons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]coupons.Coupon, 0)
	for rows.Next() {
		coupon, scanErr := scanCoupon(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if activeOnly && coupon.ExpiresAt != nil && !coupon.ExpiresAt.After(now) {
			continue
		}
		result = append(result, coupon)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate coupons: %w", err)
	}
	return result, nil
}

// RedeemCoupon atomically creates a wallet grant or applies an immediate balance effect.
