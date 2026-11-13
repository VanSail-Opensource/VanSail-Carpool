package database

import (
	"context"
	"database/sql"
	"time"
)

const completedOutboxRetention = 7 * 24 * time.Hour

func pruneCompletedOutboxTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	expired, err := expiredTimestampIDsTx(ctx, tx, `SELECT job.id,job.updated_at FROM outbox_jobs job WHERE job.status='done' AND job.updated_at<$1
 AND NOT EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
 WHERE item.target_type='outbox_job' AND item.target_id=job.id AND operation.status IN ('queued','processing','pending_review','partial'))`, now.Add(-completedOutboxRetention))
	if err != nil {
		return err
	}
	count, err := pruneTimestampIDsTx(ctx, tx, `DELETE FROM outbox_jobs WHERE id=ANY($1) AND status='done'`, expired)
	if err != nil {
		return err
	}
	counts["completed_outbox_jobs"] += count
	return nil
}
