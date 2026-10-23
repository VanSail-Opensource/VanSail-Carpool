package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// claimableOutboxTx skips occupied resources without holding a provider call or
// mutating unrelated jobs. Resource keys are derived from existing durable facts.
func claimableOutboxTx(ctx context.Context, tx *sql.Tx, now time.Time, priority bool) (*model.OutboxJob, error) {
	active, err := outboxBatchTx(ctx, tx, outboxSelect+` WHERE status='processing'`)
	if err != nil {
		return nil, err
	}
	occupied := map[string]bool{}
	for _, job := range active {
		keys, err := outboxResourcesTx(ctx, tx, job)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			occupied[key] = true
		}
	}
	const batchSize = 64
	for offset := 0; ; offset += batchSize {
		jobs, err := outboxBatchTx(ctx, tx, outboxSelect+` WHERE status='pending' AND available_at<=? AND (?=0 OR (`+priorityOutboxFilter+`))
   AND (kind NOT IN ('ip_lookup_report_retry','ip_lookup_live_retry') OR (
    NOT EXISTS(SELECT 1 FROM ip_lookup_reports WHERE status='processing')
    AND NOT EXISTS(SELECT 1 FROM outbox_jobs active WHERE active.kind IN ('ip_lookup_report_retry','ip_lookup_live_retry') AND active.status='processing')))
   ORDER BY CASE WHEN `+priorityOutboxFilter+` THEN 0 ELSE 1 END,available_at,created_at,id LIMIT ? OFFSET ?`, stamp(now), boolInt(priority), batchSize, offset)
		if err != nil {
			return nil, err
		}
		for _, job := range jobs {
			keys, err := outboxResourcesTx(ctx, tx, job)
			if err != nil {
				return nil, err
			}
			blocked := false
			for _, key := range keys {
				blocked = blocked || occupied[key]
			}
			// A blocked predecessor also reserves its other resources, preserving order
			// for multi-account operations while unrelated work can still advance.
			if blocked {
				for _, key := range keys {
					occupied[key] = true
				}
				continue
			}
			return &job, nil
		}
		if len(jobs) < batchSize {
			return nil, nil
		}
	}
}

func outboxBatchTx(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]model.OutboxJob, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []model.OutboxJob{}
	for rows.Next() {
		job, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
