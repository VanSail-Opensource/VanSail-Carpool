package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

func (s *Store) ClaimOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanOutbox(tx.QueryRowContext(ctx, outboxSelect+` WHERE status='pending' AND available_at<=?
		AND (kind NOT IN ('ip_lookup_report_retry','ip_lookup_live_retry') OR (
			NOT EXISTS(SELECT 1 FROM ip_lookup_reports WHERE status='processing')
			AND NOT EXISTS(SELECT 1 FROM outbox_jobs active WHERE active.kind IN ('ip_lookup_report_retry','ip_lookup_live_retry') AND active.status='processing')))
		ORDER BY available_at,created_at LIMIT 1`, stamp(now)))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='processing',attempts=attempts+1,updated_at=? WHERE id=? AND status='pending'`, stamp(now), job.ID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return nil, nil
	}
	job.Status = "processing"
	job.Attempts++
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &job, nil
}

// CompleteOutboxJob removes successful work or schedules bounded exponential retry.

func (s *Store) CompleteOutboxJob(ctx context.Context, jobID string, attempts int, jobErr error, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if jobErr == nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=?`, jobID); err != nil {
			return fmt.Errorf("remove completed outbox job: %w", err)
		}
		return tx.Commit()
	}
	status := "pending"
	if attempts >= 10 {
		status = "failed"
	}
	delay := time.Minute << min(attempts-1, 8)
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

func (s *Store) RetryOutboxJob(ctx context.Context, jobID string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
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

func (s *Store) DeleteOutboxJob(ctx context.Context, jobID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
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
