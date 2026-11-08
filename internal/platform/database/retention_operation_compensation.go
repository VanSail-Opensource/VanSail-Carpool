package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type staleDebit struct {
	referenceID string
	userID      string
	amount      int64
}

func compensateStaleOperationDebitsTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	if err := retainAttemptedEmbySetupsTx(ctx, tx); err != nil {
		return err
	}
	if err := compensateStaleTrafficResetsTx(ctx, tx, now); err != nil {
		return err
	}
	return compensateStaleEmbySetupsTx(ctx, tx, now)
}

// Legacy provisioning has its own durable attempt marker. Match setup's
// balance-before-account order, then prevent a racing create marker/refund.
func retainAttemptedEmbySetupsTx(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`SELECT b.user_id FROM balances b WHERE EXISTS(SELECT 1 FROM maintenance_operation_candidates c
 JOIN provider_operations p ON p.id=c.id WHERE p.kind='emby_setup' AND p.owner_user_id=b.user_id)
 ORDER BY b.user_id FOR UPDATE OF b`,
		`SELECT a.id FROM emby_accounts a WHERE EXISTS(
 SELECT 1 FROM maintenance_operation_candidates c JOIN provider_operations p ON p.id=c.id
 WHERE p.kind='emby_setup' AND p.owner_user_id=a.user_id) ORDER BY a.id FOR UPDATE OF a`,
	}
	for _, query := range queries {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err = finishCompileRows(rows); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM maintenance_operation_candidates c WHERE EXISTS(
 SELECT 1 FROM provider_operations p JOIN emby_accounts a ON a.user_id=p.owner_user_id
 WHERE p.id=c.id AND p.kind='emby_setup' AND (a.create_attempted<>0 OR COALESCE(a.remote_user_id,'')<>''))`)
	return err
}

func compensateStaleTrafficResetsTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT operation.id,debit.user_id,-debit.delta_txb_minor
		FROM maintenance_operation_candidates candidate
		JOIN provider_operations operation ON operation.id=candidate.id AND operation.kind='purchase_traffic_reset'
		JOIN ledger_entries debit ON debit.kind='traffic_reset_debit' AND debit.reference_id=operation.id
		WHERE debit.delta_txb_minor<0 AND NOT EXISTS (SELECT 1 FROM ledger_entries compensation
			WHERE compensation.kind='traffic_reset_compensation' AND compensation.reference_id=operation.id)`)
	if err != nil {
		return fmt.Errorf("load stale traffic reset debits: %w", err)
	}
	debits, err := collectStaleDebits(rows)
	if err != nil {
		return err
	}
	for _, debit := range debits {
		balance, changeErr := changeBalanceTx(ctx, tx, debit.userID, debit.amount, now)
		if changeErr != nil {
			return fmt.Errorf("refund stale traffic reset: %w", changeErr)
		}
		if _, changeErr = insertLedgerTx(ctx, tx, debit.userID, debit.amount, balance,
			"traffic_reset_compensation", debit.referenceID, "stale traffic reset compensation", now); changeErr != nil {
			return changeErr
		}
	}
	return nil
}

func compensateStaleEmbySetupsTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT account.id || ':' || account.setup_attempt,account.user_id,account.setup_price_txb_minor
		FROM maintenance_operation_candidates candidate
		JOIN provider_operations operation ON operation.id=candidate.id AND operation.kind='emby_setup'
		JOIN emby_accounts account ON account.user_id=operation.owner_user_id
		WHERE account.status IN ('queued','provisioning','pending_review') AND account.refunded_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM ledger_entries refund WHERE refund.kind='emby_setup_refund'
			AND refund.reference_id=account.id || ':' || account.setup_attempt)`)
	if err != nil {
		return fmt.Errorf("load stale Emby setup debits: %w", err)
	}
	debits, err := collectStaleDebits(rows)
	if err != nil {
		return err
	}
	for _, debit := range debits {
		balance, changeErr := changeBalanceTx(ctx, tx, debit.userID, debit.amount, now)
		if changeErr != nil {
			return fmt.Errorf("refund stale Emby setup: %w", changeErr)
		}
		if _, changeErr = insertLedgerTx(ctx, tx, debit.userID, debit.amount, balance,
			"emby_setup_refund", debit.referenceID, "stale Emby setup refund", now); changeErr != nil {
			return changeErr
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE emby_accounts SET status='failed',password_ciphertext='',password_context='',
		pending_preferences_json='{}',last_error='maintenance timeout',refunded_at=$1,updated_at=$2
		WHERE user_id IN (SELECT operation.owner_user_id FROM maintenance_operation_candidates candidate
			JOIN provider_operations operation ON operation.id=candidate.id AND operation.kind='emby_setup')
		AND status IN ('queued','provisioning','pending_review') AND refunded_at IS NULL`, stamp(now), stamp(now))
	return err
}

func collectStaleDebits(rows *sql.Rows) ([]staleDebit, error) {
	defer func() { _ = rows.Close() }()
	result := make([]staleDebit, 0)
	for rows.Next() {
		var debit staleDebit
		if err := rows.Scan(&debit.referenceID, &debit.userID, &debit.amount); err != nil {
			return nil, err
		}
		result = append(result, debit)
	}
	return result, rows.Err()
}
