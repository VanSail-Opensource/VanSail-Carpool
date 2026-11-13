package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) ClaimOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return s.claimOutboxJob(ctx, now, false)
}

// ClaimPriorityOutboxJob reserves dispatch capacity for dynamic firewall intent
// while sharing the same transaction, lease and retry records as ordinary work.
func (s *Store) ClaimPriorityOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return s.claimOutboxJob(ctx, now, true)
}

func (s *Store) ClaimComputeOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return s.claimOutboxJob(ctx, now, false, true)
}

const priorityOutboxFilter = `kind='xnode_firewall' AND (CASE ((CASE WHEN COALESCE((payload) IS JSON,false) THEN payload ELSE '{}' END)::jsonb #> '{bank}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (CASE WHEN COALESCE((payload) IS JSON,false) THEN payload ELSE '{}' END)::jsonb #>> '{bank}' END)='dynamic'`

func (s *Store) retryAttempt_claimOutboxJob(ctx context.Context, now time.Time, priorityOnly bool, computeOnly ...bool) (*model.OutboxJob, error) {

	var options *sql.TxOptions

	options = &sql.TxOptions{Isolation: sql.LevelReadCommitted}

	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(72173681)`); err != nil {
		return nil, err
	}

	job, err := claimableOutboxTx(ctx, tx, now, priorityOnly, computeOnly...)
	if err != nil || job == nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='processing',attempts=attempts+1,updated_at=$1 ,claim_generation=claim_generation+1 WHERE id=$2 AND status='pending'`, stamp(now), job.ID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return nil, nil
	}
	job.Status = "processing"
	job.Attempts++

	job.ClaimGeneration++

	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

// RetryOutboxJob lets an administrator explicitly retry a failed operation.

func (s *Store) retryAttempt_RetryOutboxJob(ctx context.Context, jobID string, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM outbox_jobs WHERE id=$1 AND status='failed'`, jobID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConflict
		}
		return err
	}
	if err := reactivateOutboxJobTx(ctx, tx, jobID, kind, now); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteOutboxJob removes an unleased job only when it is not required for
// financial settlement, an unfinished command or restriction recovery.

func (s *Store) retryAttempt_DeleteOutboxJob(ctx context.Context, jobID, actorID string) error {
	if actorID == "" {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	var kind, status string
	if err := tx.QueryRowContext(ctx, `SELECT kind,status FROM outbox_jobs WHERE id=$1 FOR UPDATE`, jobID).Scan(&kind, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status == "processing" {
		return ErrConflict
	}
	now := time.Now().UTC()
	if required, err := outboxDeletionRequiresRecoveryTx(ctx, tx, jobID, now); err != nil {
		return err
	} else if required {
		return ErrConflict
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=$1 AND status<>'processing'`, jobID)
	if err != nil {
		return fmt.Errorf("delete outbox job: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return fmt.Errorf("inspect deleted outbox job: %w", rowsErr)
	} else if affected == 0 {
		return ErrConflict
	}
	if err := auditOutboxDeletionTx(ctx, tx, actorID, jobID, kind, status, now); err != nil {
		return err
	}
	return tx.Commit()
}
