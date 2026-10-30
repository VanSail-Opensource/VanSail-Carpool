package database

import (
	"context"
	"database/sql"
	"time"
)

const completedOutboxRetention = 7 * 24 * time.Hour

// PruneTransientRecords retains the legacy entry point for retry artifacts.
// Scheduled cleanup shares its helper inside the backup-gated transaction.
func (s *Store) retryAttempt_PruneTransientRecords(ctx context.Context, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneCompletedOutboxTx(ctx, tx, now.UTC(), map[string]int64{}); err != nil {
		return err
	}
	return tx.Commit()
}

func pruneCompletedOutboxTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	expired, err := expiredTimestampIDsTx(ctx, tx, `SELECT job.id,job.updated_at FROM outbox_jobs job WHERE job.status='done' AND job.updated_at<?
 AND NOT EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
 WHERE item.target_type='outbox_job' AND item.target_id=job.id AND operation.status IN ('queued','processing','pending_review','partial'))`, now.Add(-completedOutboxRetention))
	if err != nil {
		return err
	}
	for _, id := range expired {
		count, err := deleteCount(ctx, tx, `DELETE FROM outbox_jobs WHERE id=? AND status='done'`, id)
		if err != nil {
			return err
		}
		counts["completed_outbox_jobs"] += count
	}
	return nil
}
