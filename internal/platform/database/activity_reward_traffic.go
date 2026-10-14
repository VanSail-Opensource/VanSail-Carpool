package database

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// Keep NULL overrides intact until a recurring grant actually changes the base.
func drawRenewalTrafficTx(ctx context.Context, tx *sql.Tx, purchaseID string) (sql.NullInt64, int64, error) {
	var override sql.NullInt64
	var base int64
	err := tx.QueryRowContext(ctx, `SELECT CASE WHEN p.reward_traffic_renewal=0 THEN p.reward_renewal_traffic_limit_bytes
		ELSE p.entitlement_traffic_limit_bytes END,c.traffic_limit_bytes
		FROM purchases p JOIN combos c ON c.id=p.combo_id WHERE p.id=?`, purchaseID).Scan(&override, &base)
	if override.Valid {
		base = override.Int64
	}
	return override, base, err
}

func drawTrafficAfterDelta(traffic, delta int64) (int64, error) {
	if traffic <= 0 || delta < 0 && delta <= -traffic || delta > 0 && traffic > math.MaxInt64-delta {
		return 0, ErrConflict
	}
	return traffic + delta, nil
}

func applyDrawTrafficTx(ctx context.Context, tx *sql.Tx, purchaseID string, traffic, value int64, includeInRenewal bool, now time.Time) error {
	if value > math.MaxInt64/(1<<30) || value < math.MinInt64/(1<<30) {
		return activity.ErrInvalidInput
	}
	delta := value * (1 << 30)
	next, err := drawTrafficAfterDelta(traffic, delta)
	if err != nil {
		return err
	}
	renewalOverride, renewalBase, err := drawRenewalTrafficTx(ctx, tx, purchaseID)
	if err != nil {
		return err
	}
	if includeInRenewal {
		renewalBase, err = drawTrafficAfterDelta(renewalBase, delta)
		if err != nil {
			return err
		}
		renewalOverride = sql.NullInt64{Int64: renewalBase, Valid: true}
	}
	// A recurring reward changes only the renewal base, never promotes earlier
	// one-term traffic gains or losses into future terms.
	_, err = tx.ExecContext(ctx, `UPDATE purchases SET entitlement_traffic_limit_bytes=?,reward_traffic_renewal=?,
		reward_renewal_traffic_limit_bytes=?,updated_at=? WHERE id=?`,
		next, boolInt(includeInRenewal && next == renewalBase), renewalOverride, stamp(now), purchaseID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE purchase_rollovers SET traffic_limit_bytes=?,updated_at=? WHERE purchase_id=? AND status='pending'`, next, stamp(now), purchaseID)
	return err
}
