package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// RestoreXNodePurchasePeriod updates or reopens the same retained paid period.
// The current purchase, pointer and closure generation are checked in the write
// transaction; usage and the reset origin never come from the caller's snapshot.
func (s *Store) retryAttempt_RestoreXNodePurchasePeriod(ctx context.Context, identity xnode.Identity, expected xnode.Period, purchase model.Purchase, generation int64, now time.Time) error {
	if identity.UserID == "" || identity.ServiceName != "" || identity.CurrentPeriodID != expected.ID ||
		expected.ID == "" || expected.ProxyUserID != identity.ProxyUserID || expected.PurchaseID != purchase.ID || purchase.UserID != identity.UserID || generation < 0 {
		return xnode.ErrConflict
	}
	if expected.Status != "open" && expected.Status != "closing" && expected.Status != "closed" {
		return xnode.ErrAccessAccountingPending
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockNativeMemberIdentityTx(ctx, tx, identity.UserID, generation, true); err != nil {
		return err
	}
	if err = lockMemberPurchaseTx(ctx, tx, identity.UserID); err != nil {
		return err
	}
	if err = validatePurchasePeriodTargetTx(ctx, tx, purchase, now); err != nil {
		return err
	}
	var pointer string
	var disabled bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(current_period_id,''),access_disabled=1 FROM xnode_identities
		WHERE proxy_user_id=$1 AND user_id=$2 AND service_name IS NULL FOR UPDATE`, identity.ProxyUserID, identity.UserID).Scan(&pointer, &disabled)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (disabled || pointer != expected.ID) {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	current, err := scanXNodePeriod(tx.QueryRowContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods WHERE id=$1 FOR UPDATE`, expected.ID))
	if errors.Is(err, xnode.ErrNotFound) {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	if current.ProxyUserID != expected.ProxyUserID || current.PurchaseID != expected.PurchaseID || current.Status != expected.Status || current.ClosureGeneration != expected.ClosureGeneration {
		return xnode.ErrConflict
	}
	if _, err = enabledSquadsTx(ctx, tx, identity.UserID, purchase.SquadUUIDs); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET limit_bytes=$1,strategy=$2,status='open',ended_at=NULL WHERE id=$3`, purchase.TrafficLimitBytes, purchase.ResetStrategy, current.ID); err != nil {
		return err
	}
	if current.Status != "open" {
		// Retired-node confirmations remain authoritative; all live admission
		// participants must produce a new barrier for the next closure epoch.
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_period_nodes p SET flushed_sequence=NULL,flushed_stream_id='',flushed_legacy_sequence=0
			WHERE p.period_id=$1 AND NOT EXISTS(SELECT 1 FROM xnode_nodes n WHERE n.id=p.node_id AND n.enabled=0
			AND (n.removing_at IS NOT NULL OR n.retired_at IS NOT NULL) AND p.flushed_sequence IS NOT NULL)`, current.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
