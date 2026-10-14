package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

func (s *Store) RecoverOutbox(ctx context.Context, staleBefore, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',available_at=?,last_error='worker interrupted',updated_at=? WHERE status='processing' AND updated_at<?`, stamp(now), stamp(now), stamp(staleBefore))
	if err != nil {
		return fmt.Errorf("recover outbox leases: %w", err)
	}
	return nil
}

// ListOutboxJobs returns recent synchronization state.

func (s *Store) ListOutboxJobs(ctx context.Context, limit int) ([]model.OutboxJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, outboxSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]model.OutboxJob, 0)
	for rows.Next() {
		job, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

const outboxSelect = `SELECT id,kind,payload,status,attempts,available_at,last_error,created_at,updated_at FROM outbox_jobs`

func scanOutbox(row rowScanner) (model.OutboxJob, error) {
	var job model.OutboxJob
	var available, created, updated string
	if err := row.Scan(&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &available, &job.LastError, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.OutboxJob{}, ErrNotFound
		}
		return model.OutboxJob{}, err
	}
	var err error
	if job.AvailableAt, err = parseStamp(available); err != nil {
		return model.OutboxJob{}, err
	}
	if job.CreatedAt, err = parseStamp(created); err != nil {
		return model.OutboxJob{}, err
	}
	job.UpdatedAt, err = parseStamp(updated)
	return job, err
}

func sanitizeError(err error) string {
	value := err.Error()
	if len(value) > 500 {
		return value[:500]
	}
	return value
}
