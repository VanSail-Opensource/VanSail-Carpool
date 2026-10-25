package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const questionnaireImportRetention = 7 * 24 * time.Hour
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
	if err := pruneAbandonedImportsTx(ctx, tx, now.UTC(), map[string]int64{}); err != nil {
		return err
	}
	return tx.Commit()
}

func pruneAbandonedImportsTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	ids, err := expiredTimestampIDsTx(ctx, tx, `SELECT upload.id,upload.updated_at FROM questionnaire_imports upload
 WHERE upload.status IN ('preview','failed') AND upload.updated_at<?
 AND NOT EXISTS(SELECT 1 FROM questionnaire_imports other WHERE other.questionnaire_id=upload.questionnaire_id AND other.status IN ('queued','processing'))
 AND NOT EXISTS(SELECT 1 FROM outbox_jobs job WHERE job.kind='questionnaire_settlement'
 AND json_extract(job.payload,'$.importId')=upload.id AND (job.status IN ('pending','processing')
 OR EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
 WHERE item.target_type='outbox_job' AND item.target_id=job.id AND operation.kind='outbox_retry'
 AND operation.status IN ('queued','processing','pending_review','partial'))))`, now.Add(-questionnaireImportRetention))
	if err != nil {
		return fmt.Errorf("select abandoned questionnaire imports: %w", err)
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='closed',updated_at=?
   WHERE status='settling' AND id=(SELECT questionnaire_id FROM questionnaire_imports WHERE id=? AND status='failed')`, stamp(now), id); err != nil {
			return err
		}
		count, err := deleteCount(ctx, tx, `DELETE FROM outbox_jobs WHERE kind='questionnaire_settlement'
   AND status IN ('failed','done') AND json_extract(payload,'$.importId')=?`, id)
		if err != nil {
			return err
		}
		counts["questionnaire_jobs"] += count
		count, err = deleteCount(ctx, tx, `DELETE FROM questionnaire_imports WHERE id=?`, id)
		if err != nil {
			return err
		}
		counts["questionnaire_imports"] += count
	}
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
