package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) failAutoRenewalTx(ctx context.Context, tx *sql.Tx, purchaseID, userID, reason string, now time.Time) (bool, error) {
	var rawUntil string
	err := tx.QueryRowContext(ctx, `SELECT valid_until FROM purchases WHERE id=$1 AND user_id=$2
		AND auto_renew_enabled=1 AND status IN ('active','activating','expired') FOR UPDATE`, purchaseID, userID).Scan(&rawUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	until, err := parseStamp(rawUntil)
	if err != nil || until.After(now) {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='expired',auto_renew_enabled=0,auto_renew_failure_reason=$1,auto_renew_failed_at=$2,updated_at=$3 WHERE id=$4 AND user_id=$5 AND auto_renew_enabled=1 AND valid_until=$6 AND status IN ('active','activating','expired')`, reason, stamp(now), stamp(now), purchaseID, userID, rawUntil)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	if reason == AutoRenewalReasonInsufficientBalance {
		if _, err = tx.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET outcome_reason='INSUFFICIENT_FUNDS' WHERE purchase_id=$1 AND state<>'SETTLED'`, purchaseID); err != nil {
			return false, err
		}
	}
	if err = insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
		return false, err
	}
	return true, s.insertAutoRenewalFailureNoticeTx(ctx, tx, purchaseID, userID, reason, userSyncGate(userID), now)
}
