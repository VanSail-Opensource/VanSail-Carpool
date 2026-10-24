package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) failCalculatedAutoRenewalTx(ctx context.Context, tx *sql.Tx, purchaseID, userID, reason string, now time.Time) (bool, error) {
	result, err := tx.ExecContext(ctx, `UPDATE purchase_rollovers SET status='zero',credited_txb_minor=0,updated_at=?,completed_at=?
		WHERE purchase_id=? AND status='calculated'`, stamp(now), stamp(now), purchaseID)
	if err != nil {
		return false, fmt.Errorf("complete failed automatic rollover: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		result, updateErr := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=0,auto_renew_failure_reason=?,auto_renew_failed_at=?,updated_at=?
			WHERE id=? AND status='expired' AND auto_renew_enabled=1`, reason, stamp(now), stamp(now), purchaseID)
		if updateErr != nil {
			return false, fmt.Errorf("record resumed automatic renewal failure: %w", updateErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return false, rowsErr
		}
		if rows != 1 {
			return false, nil
		}
		return true, s.insertAutoRenewalFailureNoticeTx(ctx, tx, purchaseID, userID, reason, "", now)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',auto_renew_enabled=0,auto_renew_failure_reason=?,auto_renew_failed_at=?,updated_at=?
		WHERE id=? AND status IN ('active','activating')`, reason, stamp(now), stamp(now), purchaseID); err != nil {
		return false, fmt.Errorf("expire failed automatic renewal: %w", err)
	}
	if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
		return false, err
	}
	if err := s.insertAutoRenewalFailureNoticeTx(ctx, tx, purchaseID, userID, reason, userSyncGate(userID), now); err != nil {
		return false, err
	}
	return true, nil
}
