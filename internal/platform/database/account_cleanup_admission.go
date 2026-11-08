package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// AccountCleanupAdmissionValid rechecks exact current-generation final evidence.
// Callers performing SQL mutations use the transaction helper before other locks.
func (s *Store) AccountCleanupAdmissionValid(ctx context.Context, id string, generation int64, at time.Time) (bool, error) {
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) { return s.accountCleanupAdmissionTx(ctx, tx, id, generation, at) })
}

func (s *Store) accountCleanupAdmissionTx(ctx context.Context, tx *sql.Tx, id string, generation int64, at time.Time) (bool, error) {
	ok, _, err := s.accountCleanupAdmissionStatusTx(ctx, tx, id, generation, at)
	return ok, err
}

// AccountCleanupAdmissionReason explains a genuine dependency without changing
// accounting, delivery retries, candidate evidence or user lifecycle state.
func (s *Store) AccountCleanupAdmissionReason(ctx context.Context, id string, generation int64, at time.Time) (string, error) {
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (string, error) {
		_, reason, err := s.accountCleanupAdmissionStatusTx(ctx, tx, id, generation, at)
		return reason, err
	})
}

func (s *Store) accountCleanupAdmissionStatusTx(ctx context.Context, tx *sql.Tx, id string, generation int64, at time.Time) (bool, string, error) {
	h, err := lockCleanupHealth(ctx, tx, false)
	if err != nil {
		return false, "", err
	}
	var current, telegram int64
	var role, status string
	err = tx.QueryRowContext(ctx, `SELECT account_generation,telegram_id,role,account_status FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&current, &telegram, &role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "account_superseded", nil
	}
	if err != nil {
		return false, "", err
	}
	if s.cleanupProtected(telegram, role) {
		return false, "administrator_protected", nil
	}
	if current != generation || status != "purging" {
		return false, "account_superseded", nil
	}
	if !cleanupVerified(h, at) {
		return false, "bot_unverified", nil
	}
	if !cleanupHealthy(h, at) {
		return false, "correlated_failure_pause", nil
	}
	c, err := readCleanupCheck(ctx, tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "seven_day_evidence_required", nil
	}
	if err != nil {
		return false, "", err
	}
	// A queued timestamp cannot extend freshness while SQL waits for locks.
	var databaseNow time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow); err != nil {
		return false, "", err
	}
	if databaseNow.After(at) {
		at = databaseNow
	}
	if !cleanupVerified(h, at) {
		return false, "bot_unverified", nil
	}
	if !cleanupHealthy(h, at) {
		return false, "correlated_failure_pause", nil
	}
	if c.epoch != h.Epoch {
		return false, "bot_epoch_changed", nil
	}
	if !c.admission.Valid || !cleanupMature(c, at) {
		return false, "seven_day_evidence_required", nil
	}
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_cleanup_evidence WHERE nonce=$1 AND user_id=$2 AND account_generation=$3 AND bot_id=$4 AND bot_epoch=$5 AND source='final' AND qualifying=1 AND purge_generation=$6 AND completed_at<=$7 AND completed_at>=$8)`, c.admission.String, id, c.generation, h.BotID, h.Epoch, generation, at.UTC(), at.Add(-cleanupFinalTTL)).Scan(&valid)
	if err != nil {
		return false, "", err
	}
	if !valid {
		return false, "fresh_final_check_required", nil
	}
	return true, "", nil
}
