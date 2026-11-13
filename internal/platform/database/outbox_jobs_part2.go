package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Store) RecoverOutbox(ctx context.Context, staleBefore, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',available_at=$1,last_error='worker interrupted',updated_at=$2 ,claim_generation=claim_generation+1 WHERE status='processing' AND updated_at<$3 AND `+purchaseTimestampOrderSQL("updated_at")+`<$4`, stamp(now), stamp(now), retentionCandidateUpperBound(staleBefore), purchaseTimestampOrder(staleBefore))
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
	rows, err := s.db.QueryContext(ctx, outboxSelect+` ORDER BY `+purchaseTimestampOrderSQL("created_at")+` DESC,record_order DESC LIMIT $1`, limit)
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

const outboxSelect = `SELECT id,kind,payload,status,attempts,available_at,last_error,created_at,updated_at,claim_generation FROM outbox_jobs`

func scanOutbox(row rowScanner) (model.OutboxJob, error) {
	var job model.OutboxJob
	var available, created, updated string
	if err := row.Scan(&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &available, &job.LastError, &created, &updated, &job.ClaimGeneration); err != nil {
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
	return truncateErrorText(err.Error())
}

// Error evidence must remain valid UTF-8 after the byte cap: PostgreSQL rejects
// a split multibyte character, which would otherwise prevent claim completion.
func truncateErrorText(value string) string {
	value = strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", "\uFFFD"), "\uFFFD")
	if len(value) > 500 {
		value = value[:500]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}
