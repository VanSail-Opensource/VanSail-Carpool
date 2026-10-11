package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const maintenanceTimeoutCode = "MAINTENANCE_TIMEOUT"

func pruneProviderOperationsTx(ctx context.Context, tx *sql.Tx, cutoff, now time.Time,
	counts map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_operation_candidates(id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create stale operation set: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO maintenance_operation_candidates(id)
		SELECT id FROM provider_operations operation WHERE status IN ('queued','processing','pending_review','partial') AND created_at<?
		AND kind<>'node_compensation'
		AND (kind NOT LIKE 'telegram_pm_%' OR created_at<=?) AND NOT (`+liveComboControlPredicate+`)`, stamp(cutoff), stamp(now.Add(-pmReceiptRetention)), stamp(now), stamp(now), stamp(now), stamp(now))
	if err != nil {
		return fmt.Errorf("select stale operations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_operation_candidates WHERE id IN (SELECT c.operation_id FROM ip_lookup_checks c JOIN ip_lookup_reports r ON r.id=c.report_id WHERE r.status='processing' AND julianday(r.heartbeat_at)>julianday(?))`, stamp(now.Add(-2*time.Minute))); err != nil {
		return err
	}
	if err := reconcileExpiredPMOperationsTx(ctx, tx, now, counts); err != nil {
		return err
	}
	if err := reconcileStaleIPLookupsTx(ctx, tx, now); err != nil {
		return err
	}
	if err := compensateStaleOperationDebitsTx(ctx, tx, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='failed',provider_payload='{}',payment_url=NULL,
		qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=?
		WHERE status='creating' AND id IN (SELECT item.target_id FROM provider_operation_items item
			JOIN maintenance_operation_candidates candidate ON candidate.id=item.operation_id
			JOIN provider_operations operation ON operation.id=candidate.id AND operation.kind='payment_create'
			WHERE item.target_type='payment_order')`, stamp(now)); err != nil {
		return fmt.Errorf("fail stale payment creation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE connection_ip_blocks SET status='pending_review',updated_at=?
		WHERE status IN ('blocking','unblocking') AND (block_operation_id IN (SELECT id FROM maintenance_operation_candidates)
			OR unblock_operation_id IN (SELECT id FROM maintenance_operation_candidates))`, stamp(now)); err != nil {
		return fmt.Errorf("flag stale IP block operation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='failed',error_code=?,completed_at=?,updated_at=?
		WHERE operation_id IN (SELECT id FROM maintenance_operation_candidates)
		AND status IN ('queued','processing','pending_review')`, maintenanceTimeoutCode, stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("fail stale operation items: %w", err)
	}
	if counts["stale_provider_operations"], err = deleteCount(ctx, tx, `UPDATE provider_operations SET status='failed',
		error_code=?,completed_at=?,updated_at=? WHERE id IN (SELECT id FROM maintenance_operation_candidates)`,
		maintenanceTimeoutCode, stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("fail stale operations: %w", err)
	}
	if counts["provider_operation_jobs"], err = deleteCount(ctx, tx, `DELETE FROM outbox_jobs WHERE kind='provider_operation'
		AND json_extract(payload,'$.operationId') IN (SELECT id FROM provider_operations operation
			WHERE status IN ('succeeded','failed','compensated') AND (kind NOT LIKE 'telegram_pm_%' OR created_at<=?)
			AND NOT (status='failed' AND (`+liveComboControlPredicate+`)))`, stamp(now.Add(-pmReceiptRetention)), stamp(now), stamp(now), stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("prune provider operation jobs: %w", err)
	}
	// Keep terminal operations that durable domain records still use as their
	// audit or state-transition link. Deleting them would violate foreign keys.
	if counts["provider_operations"], err = deleteCount(ctx, tx, `DELETE FROM provider_operations AS operation
		WHERE status IN ('succeeded','failed','compensated')
		AND (kind NOT LIKE 'telegram_pm_%' OR created_at<=?)
		AND NOT (status='failed' AND (`+liveComboControlPredicate+`))
		AND NOT EXISTS (SELECT 1 FROM admin_temporary_bans ban
			WHERE ban.ban_operation_id=operation.id OR ban.unban_operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM node_compensation_events event
			WHERE event.provider_operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM ip_lookup_checks check_record WHERE check_record.operation_id=operation.id)`, stamp(now.Add(-pmReceiptRetention)), stamp(now), stamp(now), stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("prune processed provider operations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE maintenance_operation_candidates`); err != nil {
		return fmt.Errorf("drop stale operation set: %w", err)
	}
	return nil
}
