package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

// Completion reads only the locked claim. READ COMMITTED lets heartbeats and
// unrelated queue admissions advance without serializable dependency cycles.
// Business transactions retain their serializable default and retry policy.
func (s *Store) retryAttempt_completeOutboxClaim(ctx context.Context, jobID string, attempts int, generation int64, jobErr error, now time.Time) error {
	var options *sql.TxOptions
	if IsPostgres(s.db) {
		options = &sql.TxOptions{Isolation: sql.LevelReadCommitted}
	} else {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var currentAttempts int
	var currentStatus string
	var currentGeneration int64
	if err = tx.QueryRowContext(ctx, `SELECT attempts,status,/*pg:claim-generation*/0 FROM outbox_jobs WHERE id=? /*pg:FOR UPDATE*/`, jobID).Scan(&currentAttempts, &currentStatus, &currentGeneration); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	// An older asynchronous attempt cannot remove or overwrite a recovered lease.
	if currentAttempts != attempts || currentStatus != "processing" || (IsPostgres(s.db) && currentGeneration != generation) {
		return nil
	}
	if jobErr == nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=?`, jobID); err != nil {
			return fmt.Errorf("remove completed outbox job: %w", err)
		}
		return tx.Commit()
	}
	if errors.Is(jobErr, outbox.ErrBusy) {
		delay := time.Second
		var deferred interface{ RetryDelay() time.Duration }
		if errors.As(jobErr, &deferred) {
			delay = min(time.Minute, max(time.Second, deferred.RetryDelay()))
		}
		_, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=MAX(0,attempts-1),last_error=?,available_at=?,updated_at=? WHERE id=?`, sanitizeError(jobErr), stamp(now.Add(delay)), stamp(now), jobID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	status := "pending"
	if attempts >= 10 {
		status = "failed"
	}
	delay, err := outboxRetryDelayTx(ctx, tx, jobID, attempts)
	if err != nil {
		return err
	}
	message := sanitizeError(jobErr)
	if _, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status=?,last_error=?,available_at=?,updated_at=? WHERE id=?`, status, message, stamp(now.Add(delay)), stamp(now), jobID); err != nil {
		return err
	}
	if status == "failed" {
		if _, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET status='failed',last_error=?,updated_at=?
			WHERE status IN ('queued','processing') AND id=(SELECT json_extract(payload,'$.importId') FROM outbox_jobs WHERE id=? AND kind='questionnaire_settlement')`, message, stamp(now), jobID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
