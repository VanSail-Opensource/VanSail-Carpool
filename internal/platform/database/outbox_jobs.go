package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"time"
)

func (s *Store) ClaimOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return s.claimOutboxJob(ctx, now, false)
}

// ClaimPriorityOutboxJob reserves dispatch capacity for dynamic firewall intent
// while sharing the same transaction, lease and retry records as ordinary work.
func (s *Store) ClaimPriorityOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return s.claimOutboxJob(ctx, now, true)
}

const priorityOutboxFilter = `kind='xnode_firewall' AND json_extract(CASE WHEN json_valid(payload) THEN payload ELSE '{}' END,'$.bank')='dynamic'`

func (s *Store) retryAttempt_claimOutboxJob(ctx context.Context, now time.Time, priorityOnly bool) (*model.OutboxJob, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	var options *sql.TxOptions
	if IsPostgres(s.db) {
		options = &sql.TxOptions{Isolation: sql.LevelReadCommitted}
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if IsPostgres(s.db) {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(72173681)`); err != nil {
			return nil, err
		}
	}
	job, err := claimableOutboxTx(ctx, tx, now, priorityOnly)
	if err != nil || job == nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='processing',attempts=attempts+1,updated_at=? /*pg:advance-claim*/ WHERE id=? AND status='pending'`, stamp(now), job.ID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return nil, nil
	}
	job.Status = "processing"
	job.Attempts++
	if IsPostgres(s.db) {
		job.ClaimGeneration++
	}
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

// CompleteOutboxJob removes successful work or schedules bounded exponential retry.

func (s *Store) retryAttempt_completeOutboxClaim(ctx context.Context, jobID string, attempts int, generation int64, jobErr error, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
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
		_, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=MAX(0,attempts-1),available_at=?,updated_at=? WHERE id=?`, stamp(now.Add(time.Second)), stamp(now), jobID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	status := "pending"
	if attempts >= 10 {
		status = "failed"
	}
	delay := time.Minute << min(attempts-1, 8)
	var kind string
	if err = tx.QueryRowContext(ctx, `SELECT kind FROM outbox_jobs WHERE id=?`, jobID).Scan(&kind); err != nil {
		return err
	}
	if kind == "xnode_firewall" {
		delay = 5 * time.Second << min(attempts-1, 4)
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

// RetryOutboxJob lets an administrator explicitly retry a failed operation.

func (s *Store) retryAttempt_RetryOutboxJob(ctx context.Context, jobID string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM outbox_jobs WHERE id=? AND status='failed'`, jobID).Scan(&kind); err != nil {
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

// DeleteOutboxJob removes any job that is not currently leased by a worker.

func (s *Store) retryAttempt_DeleteOutboxJob(ctx context.Context, jobID string) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var kind, status string
	if err := tx.QueryRowContext(ctx, `SELECT kind,status FROM outbox_jobs WHERE id=?`, jobID).Scan(&kind, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status == "processing" {
		return ErrConflict
	}
	if kind == "questionnaire_settlement" && status != "done" {
		if err := abandonQuestionnaireJobTx(ctx, tx, jobID, time.Now().UTC()); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=? AND status<>'processing'`, jobID)
	if err != nil {
		return fmt.Errorf("delete outbox job: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return fmt.Errorf("inspect deleted outbox job: %w", rowsErr)
	} else if affected == 0 {
		return ErrConflict
	}
	return tx.Commit()
}

func abandonQuestionnaireJobTx(ctx context.Context, tx *sql.Tx, jobID string, now time.Time) error {
	var importID, questionnaireID, status string
	err := tx.QueryRowContext(ctx, `SELECT qi.id,qi.questionnaire_id,qi.status FROM questionnaire_imports qi
		JOIN outbox_jobs job ON job.id=? AND job.kind='questionnaire_settlement' AND qi.id=json_extract(job.payload,'$.importId')`, jobID).
		Scan(&importID, &questionnaireID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status == "processing" {
		return ErrConflict
	}
	if status == "settled" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='closed',updated_at=? WHERE id=? AND status='settling'`, stamp(now), questionnaireID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM questionnaire_imports WHERE id=? AND status<>'processing'`, importID)
	return err
}

// RecoverOutbox returns leases abandoned by a crashed process to the pending queue.
