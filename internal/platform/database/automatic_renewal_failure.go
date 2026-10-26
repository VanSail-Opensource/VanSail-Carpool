package database

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) failAutoRenewalTx(ctx context.Context, tx *sql.Tx, purchaseID, userID, reason string, now time.Time) (bool, error) {
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',auto_renew_enabled=0,auto_renew_failure_reason=?,auto_renew_failed_at=?,updated_at=? WHERE id=? AND user_id=? AND auto_renew_enabled=1 AND valid_until<=? AND status IN ('active','activating','expired')`, reason, stamp(now), stamp(now), purchaseID, userID, stamp(now))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	if reason == AutoRenewalReasonInsufficientBalance {
		if _, err = tx.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET outcome_reason='INSUFFICIENT_FUNDS' WHERE purchase_id=? AND state<>'SETTLED'`, purchaseID); err != nil {
			return false, err
		}
	}
	if err = insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
		return false, err
	}
	return true, s.insertAutoRenewalFailureNoticeTx(ctx, tx, purchaseID, userID, reason, userSyncGate(userID), now)
}
