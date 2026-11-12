package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const maintenanceTimeoutCode = "MAINTENANCE_TIMEOUT"

// Failed parents do not settle attempted children. Older maintenance releases
// also used a timeout as a terminal outcome, without external failure evidence.
const settledProviderOperation = `operation.status IN ('succeeded','failed','compensated')
 AND NOT (operation.status='failed' AND (
 EXISTS(SELECT 1 FROM provider_operation_items i WHERE i.operation_id=operation.id
 AND (i.status IN('processing','pending_review','partial') OR
 (i.status='queued' AND (i.attempt_started_at IS NOT NULL OR COALESCE(i.provider_reference,'')<>''))))
 OR (operation.error_code IN('MAINTENANCE_TIMEOUT','SYNC_JOB_EXPIRED')
 AND (operation.attempts>0 OR operation.attempt_started_at IS NOT NULL OR COALESCE(operation.provider_reference,'')<>''
 OR EXISTS(SELECT 1 FROM provider_operation_items i WHERE i.operation_id=operation.id
 AND (i.attempt_started_at IS NOT NULL OR COALESCE(i.provider_reference,'')<>''))))))`

func pruneProviderOperationsTx(ctx context.Context, tx *sql.Tx, cutoff, now time.Time,
	counts map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_operation_candidates(id TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create stale operation set: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO maintenance_operation_candidates(id)
		SELECT id FROM provider_operations operation WHERE status IN ('queued','processing','pending_review','partial') AND created_at<$1
		AND kind<>'node_compensation'
		AND NOT EXISTS (SELECT 1 FROM xnode_ddns_changes change WHERE change.operation_id=operation.id AND change.phase IN ('staging','routes'))
		AND (kind NOT ILIKE 'telegram_pm_%' OR created_at<=$2) AND NOT (`+liveComboControlPredicate(3)+`)`, stamp(cutoff), stamp(now.Add(-pmReceiptRetention)), stamp(now), stamp(now), stamp(now), stamp(now))
	if err != nil {
		return fmt.Errorf("select stale operations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_operation_candidates WHERE id IN (SELECT c.operation_id FROM ip_lookup_checks c JOIN ip_lookup_reports r ON r.id=c.report_id WHERE r.status='processing' AND (r.lease_owner IS NULL OR vansail_timestamp(r.heartbeat_at)>vansail_timestamp($1) OR vansail_timestamp(r.lease_expires_at)>vansail_timestamp($2)))`, stamp(now.Add(-2*time.Minute)), stamp(now)); err != nil {
		return err
	}
	if err := reconcileExpiredPMOperationsTx(ctx, tx, now, counts); err != nil {
		return err
	}
	if err := reconcileStaleIPLookupsTx(ctx, tx, now); err != nil {
		return err
	}
	// Typed PM/IP reconciliation may retain review/checkpoint evidence. Generic
	// age cleanup can fail or refund only an intent proven never to have begun.
	if err := lockUnattemptedRetentionCandidatesTx(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_operation_candidates WHERE id NOT IN
 (SELECT p.id FROM provider_operations p WHERE `+unattemptedAccountOperation+`)`); err != nil {
		return fmt.Errorf("retain attempted provider operations: %w", err)
	}
	if err := compensateStaleOperationDebitsTx(ctx, tx, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='failed',provider_payload='{}',payment_url=NULL,
		qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=$1
		WHERE status='creating' AND id IN (SELECT item.target_id FROM provider_operation_items item
			JOIN maintenance_operation_candidates candidate ON candidate.id=item.operation_id
			JOIN provider_operations operation ON operation.id=candidate.id AND operation.kind='payment_create'
			WHERE item.target_type='payment_order')`, stamp(now)); err != nil {
		return fmt.Errorf("fail stale payment creation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE connection_ip_blocks SET status='pending_review',updated_at=$1
		WHERE status IN ('blocking','unblocking') AND (block_operation_id IN (SELECT id FROM maintenance_operation_candidates)
			OR unblock_operation_id IN (SELECT id FROM maintenance_operation_candidates))`, stamp(now)); err != nil {
		return fmt.Errorf("flag stale IP block operation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='failed',error_code=$1,completed_at=$2,updated_at=$3
		WHERE operation_id IN (SELECT id FROM maintenance_operation_candidates)
		AND status IN ('queued','processing','pending_review')`, maintenanceTimeoutCode, stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("fail stale operation items: %w", err)
	}
	if counts["stale_provider_operations"], err = deleteCount(ctx, tx, `UPDATE provider_operations SET status='failed',
		error_code=$1,completed_at=$2,updated_at=$3 WHERE id IN (SELECT id FROM maintenance_operation_candidates)`,
		maintenanceTimeoutCode, stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("fail stale operations: %w", err)
	}
	if counts["provider_operation_jobs"], err = deleteCount(ctx, tx, `DELETE FROM outbox_jobs WHERE kind='provider_operation'
		AND (CASE ((payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{operationId}' END) IN (SELECT id FROM provider_operations operation
			WHERE `+settledProviderOperation+` AND (kind NOT ILIKE 'telegram_pm_%' OR created_at<=$1)
			AND NOT (status='failed' AND (`+liveComboControlPredicate(2)+`)))`, stamp(now.Add(-pmReceiptRetention)), stamp(now), stamp(now), stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("prune provider operation jobs: %w", err)
	}
	// Command keys have no expiry: their sanitized receipts and target facts are
	// the durable replay boundary even when no domain foreign key points to them.
	// Settled outbox ciphertext is removed above. Only PM delivery receipts have
	// an explicit bounded retention contract, and referenced records stay intact.
	if counts["provider_operations"], err = deleteCount(ctx, tx, `DELETE FROM provider_operations AS operation
		WHERE `+settledProviderOperation+`
		AND kind ILIKE 'telegram_pm_%' AND created_at<=$1
		AND NOT (status='failed' AND (`+liveComboControlPredicate(2)+`))
		AND NOT EXISTS (SELECT 1 FROM admin_temporary_bans ban
			WHERE ban.ban_operation_id=operation.id OR ban.unban_operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM node_compensation_events event
			WHERE event.provider_operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM xnode_routing_plans plan WHERE plan.operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM xnode_metered_probes probe WHERE probe.operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM xnode_ddns_changes change WHERE change.operation_id=operation.id)
		AND NOT EXISTS (SELECT 1 FROM ip_lookup_checks check_record WHERE check_record.operation_id=operation.id)`, stamp(now.Add(-pmReceiptRetention)), stamp(now), stamp(now), stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("prune processed provider operations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE maintenance_operation_candidates`); err != nil {
		return fmt.Errorf("drop stale operation set: %w", err)
	}
	return nil
}

func lockUnattemptedRetentionCandidatesTx(ctx context.Context, tx *sql.Tx) error {
	// Running operations may already own financial locks. Fence only the
	// never-started intents which generic maintenance can actually supersede.
	rows, err := tx.QueryContext(ctx, `SELECT p.id FROM provider_operations p
 JOIN maintenance_operation_candidates c ON c.id=p.id WHERE `+unattemptedAccountOperation+`
 ORDER BY p.id FOR UPDATE OF p`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
	}
	return finishCompileRows(rows)
}
