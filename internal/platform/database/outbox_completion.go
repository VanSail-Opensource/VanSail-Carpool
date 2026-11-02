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

	options = &sql.TxOptions{Isolation: sql.LevelReadCommitted}

	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var currentAttempts int
	var currentStatus string
	var currentGeneration int64
	if err = tx.QueryRowContext(ctx, `SELECT attempts,status,claim_generation FROM outbox_jobs WHERE id=$1 FOR UPDATE`, jobID).Scan(&currentAttempts, &currentStatus, &currentGeneration); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	// An older asynchronous attempt cannot remove or overwrite a recovered lease.
	if currentAttempts != attempts || currentStatus != "processing" || currentGeneration != generation {
		return nil
	}
	if jobErr == nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=$1`, jobID); err != nil {
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
		_, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=GREATEST(0,attempts-1),last_error=$1,available_at=$2,updated_at=$3 WHERE id=$4`, sanitizeError(jobErr), stamp(now.Add(delay)), stamp(now), jobID)
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
	if _, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status=$1,last_error=$2,available_at=$3,updated_at=$4 WHERE id=$5`, status, message, stamp(now.Add(delay)), stamp(now), jobID); err != nil {
		return err
	}
	return tx.Commit()
}
