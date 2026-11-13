package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// ActivateXNodePurchasePeriod admits a new paid period and fences its exact
// predecessor in one transaction. The retained predecessor is returned for
// journal recovery; it is never overwritten or reset by the replacement.
func (s *Store) retryAttempt_ActivateXNodePurchasePeriod(ctx context.Context, identity xnode.Identity, expected xnode.Period, purchase model.Purchase, nextID string, generation int64, now time.Time) (xnode.Period, error) {
	if identity.UserID == "" || identity.ServiceName != "" || identity.CurrentPeriodID != expected.ID || nextID == "" ||
		purchase.UserID != identity.UserID || generation < 0 || (expected.ID != "" && (expected.ProxyUserID != identity.ProxyUserID || expected.PurchaseID == purchase.ID)) {
		return xnode.Period{}, xnode.ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Period{}, err
	}
	defer tx.Rollback()
	if err = lockNativeMemberIdentityTx(ctx, tx, identity.UserID, generation, true); err != nil {
		return xnode.Period{}, err
	}
	if err = lockMemberPurchaseTx(ctx, tx, identity.UserID); err != nil {
		return xnode.Period{}, err
	}
	if err = validatePurchasePeriodTargetTx(ctx, tx, purchase, now); err != nil {
		return xnode.Period{}, err
	}
	var pointer string
	var disabled bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(current_period_id,''),access_disabled=1 FROM xnode_identities
		WHERE proxy_user_id=$1 AND user_id=$2 AND service_name IS NULL FOR UPDATE`, identity.ProxyUserID, identity.UserID).Scan(&pointer, &disabled)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (disabled || pointer != expected.ID) {
		return xnode.Period{}, xnode.ErrConflict
	}
	if err != nil {
		return xnode.Period{}, err
	}
	var retained bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_periods WHERE proxy_user_id=$1 AND purchase_id=$2)`, identity.ProxyUserID, purchase.ID).Scan(&retained); err != nil {
		return xnode.Period{}, err
	}
	if retained {
		return xnode.Period{}, xnode.ErrAccessAccountingPending
	}
	previous, err := closePurchasePredecessorTx(ctx, tx, expected, now)
	if err != nil {
		return xnode.Period{}, err
	}
	if _, err = enabledSquadsTx(ctx, tx, identity.UserID, purchase.SquadUUIDs); err != nil {
		return xnode.Period{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,purchase_id,started_at,limit_bytes,strategy,last_reset_at,status)
		VALUES($1,$2,$3,$4,$5,$6,$4,'open')`, nextID, identity.ProxyUserID, purchase.ID, stamp(now), purchase.TrafficLimitBytes, purchase.ResetStrategy); err != nil {
		return xnode.Period{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=$1 WHERE proxy_user_id=$2`, nextID, identity.ProxyUserID); err != nil {
		return xnode.Period{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Period{}, err
	}
	return previous, nil
}

func closePurchasePredecessorTx(ctx context.Context, tx *sql.Tx, expected xnode.Period, now time.Time) (xnode.Period, error) {
	if expected.ID == "" {
		return xnode.Period{}, nil
	}
	current, err := scanXNodePeriod(tx.QueryRowContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods WHERE id=$1 FOR UPDATE`, expected.ID))
	if errors.Is(err, xnode.ErrNotFound) {
		return xnode.Period{}, xnode.ErrConflict
	}
	if err != nil {
		return xnode.Period{}, err
	}
	if current.ProxyUserID != expected.ProxyUserID || current.PurchaseID != expected.PurchaseID || current.Status != expected.Status || current.ClosureGeneration != expected.ClosureGeneration {
		return xnode.Period{}, xnode.ErrConflict
	}
	switch current.Status {
	case "closing", "closed":
		return current, nil
	case "open":
		if current.ClosureGeneration == math.MaxInt64 {
			return xnode.Period{}, xnode.ErrConflict
		}
		return scanXNodePeriod(tx.QueryRowContext(ctx, `UPDATE xnode_periods SET status='closing',ended_at=$2,closure_generation=closure_generation+1 WHERE id=$1 RETURNING `+xnodePeriodColumns, current.ID, stamp(now)))
	default:
		return xnode.Period{}, xnode.ErrAccessAccountingPending
	}
}
