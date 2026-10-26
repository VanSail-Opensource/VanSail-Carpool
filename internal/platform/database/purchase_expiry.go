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
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var user string
	var enabled int
	err = tx.QueryRowContext(ctx, `SELECT user_id,auto_renew_enabled FROM purchases WHERE id=? AND status IN ('active','activating') AND valid_until<=?`, id, stamp(now)).Scan(&user, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil || enabled == 1 {
		return err
	}
	if err = lockMemberPurchaseTx(ctx, tx, user); err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT auto_renew_enabled FROM purchases WHERE id=? AND status IN ('active','activating') AND valid_until<=?`, id, stamp(now)).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil || enabled == 1 {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE id=? AND auto_renew_enabled=0`, stamp(now), id); err != nil {
		return err
	}
	var next string
	err = tx.QueryRowContext(ctx, `SELECT id FROM purchases WHERE user_id=? AND status='queued' AND valid_from<=? AND valid_until>? ORDER BY valid_from,id LIMIT 1`, user, stamp(now), stamp(now)).Scan(&next)
	if err == nil {
		if _, err = tx.ExecContext(ctx, `UPDATE purchases SET status='activating',updated_at=? WHERE id=? AND status='queued'`, stamp(now), next); err != nil {
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
