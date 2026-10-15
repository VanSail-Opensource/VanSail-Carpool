package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const failedSyncRetention = 3 * 24 * time.Hour
const expiredSyncCode = "SYNC_JOB_EXPIRED"

// Outbound synchronization and delivery lanes are distinct from local financial
// settlement jobs, which retain their authoritative settlement/recovery state.
const synchronizationJobPredicate = `(job.kind LIKE 'remna_%' OR job.kind LIKE 'telegram_%'
	OR job.kind LIKE 'draw_telegram_%' OR job.kind IN ('emby_provision_account','connection_scan_request',
	'connection_ip_block_expiry','admin_temporary_ban_expiry',
	'draw_raffle_reply','draw_raffle_complete','draw_raffle_private','draw_instant_announcement','draw_raffle_price_notice')
	OR (job.kind='provider_operation' AND NOT EXISTS(SELECT 1 FROM provider_operations operation
		WHERE operation.id=json_extract(job.payload,'$.operationId')
		AND operation.kind IN ('questionnaire_settlement','outbox_retry','database_maintenance'))))`

func pruneFailedSynchronizationJobsTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_failed_sync_jobs(id TEXT PRIMARY KEY,kind TEXT,payload TEXT)`); err != nil {
		return err
	}
	expired, err := expiredTimestampIDsTx(ctx, tx, `SELECT job.id,job.updated_at FROM outbox_jobs job
		WHERE job.status='failed' AND job.updated_at<? AND `+synchronizationJobPredicate+`
		AND NOT EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
			WHERE item.target_type='outbox_job' AND item.target_id=job.id AND operation.kind='outbox_retry'
			AND operation.status IN ('queued','processing'))`, now.Add(-failedSyncRetention))
	if err != nil {
		return fmt.Errorf("select terminal failed synchronization jobs: %w", err)
	}
	for _, id := range expired {
		if _, err := tx.ExecContext(ctx, `INSERT INTO maintenance_failed_sync_jobs SELECT id,kind,payload FROM outbox_jobs WHERE id=?`, id); err != nil {
			return err
		}
	}
	if err := reconcileExpiredSyncStateTx(ctx, tx, now); err != nil {
		return err
	}
	if err := preserveComboControlRecoveryTx(ctx, tx, now); err != nil {
		return err
	}
	count, err := deleteCount(ctx, tx, `DELETE FROM outbox_jobs WHERE id IN (SELECT id FROM maintenance_failed_sync_jobs)`)
	if err != nil {
		return fmt.Errorf("prune terminal failed synchronization jobs: %w", err)
	}
	counts["failed_synchronization_jobs"] = count
	_, err = tx.ExecContext(ctx, `DROP TABLE maintenance_failed_sync_jobs`)
	return err
}

func reconcileExpiredSyncStateTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	// These updates record an existing failure; ownership, charges and exact reset
	// phases remain available for explicit repair, with no refund or new debit.
	queries := []struct {
		sql  string
		args []any
	}{
		{`UPDATE purchases SET status='failed',updated_at=? WHERE status='activating' AND id IN
			(SELECT json_extract(payload,'$.purchaseId') FROM maintenance_failed_sync_jobs WHERE kind='remna_apply_entitlement')`, []any{stamp(now)}},
		{`UPDATE emby_accounts SET status='pending_review',last_error=?,updated_at=? WHERE status IN ('queued','provisioning') AND id IN
			(SELECT json_extract(payload,'$.accountId') FROM maintenance_failed_sync_jobs WHERE kind='emby_provision_account')`, []any{expiredSyncCode, stamp(now)}},
		{`UPDATE connection_scans SET status=CASE WHEN status='queued' THEN 'failed' ELSE 'pending_review' END,error_code=?,updated_at=?
			WHERE status IN ('queued','processing') AND provider_job_id='' AND id IN
			(SELECT json_extract(payload,'$.scanId') FROM maintenance_failed_sync_jobs WHERE kind='connection_scan_request')`, []any{expiredSyncCode, stamp(now)}},
		{`UPDATE telegram_pm_conversations SET topic_state='pending_review',updated_at=? WHERE topic_state='creating' AND topic_operation_id IN
			(SELECT json_extract(payload,'$.operationId') FROM maintenance_failed_sync_jobs WHERE kind='provider_operation')`, []any{stamp(now)}},
		{`UPDATE telegram_pm_conversations SET profile_state='pending_review',updated_at=? WHERE profile_message_id IS NULL AND EXISTS
			(SELECT 1 FROM provider_operation_items item WHERE item.status='processing' AND item.operation_id IN
			(SELECT json_extract(payload,'$.operationId') FROM maintenance_failed_sync_jobs WHERE kind='provider_operation') AND
			((item.target_type='pm_conversation' AND item.target_id=telegram_pm_conversations.id AND item.item_key='profile') OR
			(item.target_type='pm_topic_recovery' AND item.target_id LIKE telegram_pm_conversations.id||':%')))`, []any{stamp(now)}},
		{`UPDATE provider_operation_items SET status='pending_review',error_code=CASE
			WHEN target_type='pm_inbound_message' OR target_type='pm_outbound_message' THEN 'PM_DELIVERY_UNCERTAIN'
			WHEN target_type='pm_topic_recovery' OR (target_type='pm_conversation' AND item_key='profile') THEN 'PM_PROFILE_UNCERTAIN'
			WHEN target_type='pm_conversation' AND item_key='topic' THEN 'PM_TOPIC_UNCERTAIN' ELSE ? END,completed_at=?,updated_at=?
			WHERE status='processing' AND operation_id IN (SELECT json_extract(payload,'$.operationId') FROM maintenance_failed_sync_jobs WHERE kind='provider_operation')`, []any{expiredSyncCode, stamp(now), stamp(now)}},
		{`UPDATE provider_operations SET status='pending_review',error_code=CASE
			WHEN kind LIKE 'telegram_pm_%' THEN COALESCE((SELECT error_code FROM provider_operation_items
				WHERE operation_id=provider_operations.id AND status='pending_review' AND error_code<>''
				ORDER BY CASE item_key WHEN 'relay' THEN 1 WHEN 'topic' THEN 2 ELSE 3 END LIMIT 1),?) ELSE ? END,completed_at=?,updated_at=?
			WHERE status IN ('queued','processing') AND id IN (SELECT json_extract(payload,'$.operationId') FROM maintenance_failed_sync_jobs WHERE kind='provider_operation')`, []any{expiredSyncCode, expiredSyncCode, stamp(now), stamp(now)}},
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query.sql, query.args...); err != nil {
			return fmt.Errorf("reconcile expired synchronization state: %w", err)
		}
	}
	return nil
}
