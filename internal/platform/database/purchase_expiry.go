package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ExpireDuePurchase advances the local lifecycle independently of unused bytes.
// Native periods retain their own closing/review states for later reconciliation.
func (s *Store) retryAttempt_ExpireDuePurchase(ctx context.Context, id string, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var user string
	var enabled int
	err = tx.QueryRowContext(ctx, `SELECT user_id,auto_renew_enabled FROM purchases WHERE id=$1 AND status IN ('active','activating')`, id).Scan(&user, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil || enabled == 1 {
		return err
	}
	if err = lockMemberPurchaseTx(ctx, tx, user); err != nil {
		return err
	}
	var until string
	err = tx.QueryRowContext(ctx, `SELECT auto_renew_enabled,valid_until FROM purchases WHERE id=$1 AND status IN ('active','activating')`, id).Scan(&enabled, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil || enabled == 1 {
		return err
	}
	expires, err := parseStamp(until)
	if err != nil {
		return err
	}
	if expires.After(now) {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=$1 WHERE id=$2 AND auto_renew_enabled=0`, stamp(now), id); err != nil {
		return err
	}
	next, err := dueQueuedPurchaseTx(ctx, tx, user, now)
	if err == nil {
		if _, err = tx.ExecContext(ctx, `UPDATE purchases SET status='activating',updated_at=$1 WHERE id=$2 AND status='queued'`, stamp(now), next); err != nil {
			return err
		}
		if err = applyPendingExtensionsToActivationTx(ctx, tx, next, now); err != nil {
			return err
		}
		err = insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+next+`"}`, now, now)
	} else if errors.Is(err, sql.ErrNoRows) {
		err = insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+user+`"}`, now, now)
		if err == nil {
			err = s.insertExpirationNotificationTx(ctx, tx, id, userSyncGate(user), now)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RFC3339Nano text does not sort chronologically at whole-second boundaries.
// Preserve exact paid term boundaries while selecting a queued successor.
func dueQueuedPurchaseTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases WHERE user_id=$1 AND status='queued'`, userID)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	var next string
	var earliest time.Time
	for rows.Next() {
		var id, fromText, untilText string
		if err := rows.Scan(&id, &fromText, &untilText); err != nil {
			return "", err
		}
		from, err := parseStamp(fromText)
		if err != nil {
			return "", err
		}
		until, err := parseStamp(untilText)
		if err != nil {
			return "", err
		}
		if !from.After(now) && until.After(now) && (next == "" || from.Before(earliest) || from.Equal(earliest) && id < next) {
			next, earliest = id, from
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if next == "" {
		return "", sql.ErrNoRows
	}
	return next, nil
}
