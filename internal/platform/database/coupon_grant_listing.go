package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

// ListCouponGrants returns currently usable purchase-discount grants. Expiry
// compares parsed instants: RFC3339Nano text ordering can retire a live grant.
func (s *Store) retryAttempt_ListCouponGrants(ctx context.Context, userID string, now time.Time) ([]coupons.Grant, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	expired, err := expiredTimestampIDsTx(ctx, tx, `SELECT g.id,c.expires_at FROM coupon_grants g
		JOIN coupon_definitions c ON c.id=g.coupon_id WHERE c.expires_at<$1
		AND g.user_id=$2 AND g.status='active'
		AND NOT EXISTS (SELECT 1 FROM coupon_grant_discards d WHERE d.grant_id=g.id)
		FOR UPDATE OF g,c`, now, userID)
	if err != nil {
		return nil, fmt.Errorf("find expired coupon grants: %w", err)
	}
	for _, id := range expired {
		if _, err := tx.ExecContext(ctx, `UPDATE coupon_grants SET status='expired' WHERE id=$1 AND status='active'`, id); err != nil {
			return nil, fmt.Errorf("expire coupon grant: %w", err)
		}
	}
	rows, err := tx.QueryContext(ctx, grantSelect+` WHERE coupon_grants.user_id=$1 AND coupon_grants.status='active'
		AND coupon_definitions.active=1
		AND NOT EXISTS (SELECT 1 FROM coupon_grant_discards WHERE coupon_grant_discards.grant_id=coupon_grants.id)
		AND `+authorizedCouponGrant+` ORDER BY coupon_grants.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list coupon grants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]coupons.Grant, 0)
	for rows.Next() {
		grant, scanErr := scanGrant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if grant.Coupon.ExpiresAt == nil || grant.Coupon.ExpiresAt.After(now) {
			result = append(result, grant)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
