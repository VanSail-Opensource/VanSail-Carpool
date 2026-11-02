package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Live control receipts survive transient-job retention while their owned term
// still needs exact-state synchronization. Superseded or forfeited terms cannot resume.
const liveComboControlPredicate = `operation.kind IN ('member_squad_switch','member_early_activation') AND EXISTS(
	SELECT 1 FROM provider_operation_items item JOIN purchases purchase ON purchase.id=item.target_id
	WHERE item.operation_id=operation.id AND item.target_type='purchase' AND purchase.user_id=operation.owner_user_id
	AND purchase.valid_from<=$1 AND purchase.valid_until>$2 AND
	((operation.kind='member_squad_switch' AND purchase.status='active') OR
	 (operation.kind='member_early_activation' AND purchase.status IN ('active','activating','failed')))
	AND NOT EXISTS(SELECT 1 FROM purchases other WHERE other.user_id=purchase.user_id AND other.id<>purchase.id
		AND other.status IN ('active','activating') AND other.valid_from<=$3 AND other.valid_until>$4))`

func recoverComboControlJobTx(ctx context.Context, tx *sql.Tx, jobID string, now time.Time) error {
	var operationID string
	err := tx.QueryRowContext(ctx, `SELECT operation.id FROM outbox_jobs job JOIN provider_operations operation
		ON operation.id=(CASE ((job.payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (job.payload)::jsonb #>> '{operationId}' END) WHERE job.id=$1 AND job.kind='provider_operation'
		AND operation.status IN ('failed','pending_review') AND operation.kind IN ('member_squad_switch','member_early_activation') AND EXISTS(
	SELECT 1 FROM provider_operation_items item JOIN purchases purchase ON purchase.id=item.target_id
	WHERE item.operation_id=operation.id AND item.target_type='purchase' AND purchase.user_id=operation.owner_user_id
	AND purchase.valid_from<=$2 AND purchase.valid_until>$3 AND
	((operation.kind='member_squad_switch' AND purchase.status='active') OR
	 (operation.kind='member_early_activation' AND purchase.status IN ('active','activating','failed')))
	AND NOT EXISTS(SELECT 1 FROM purchases other WHERE other.user_id=purchase.user_id AND other.id<>purchase.id
		AND other.status IN ('active','activating') AND other.valid_from<=$4 AND other.valid_until>$5))`,
		jobID, stamp(now), stamp(now), stamp(now), stamp(now)).Scan(&operationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',updated_at=$1 WHERE status='failed' AND id IN
		(SELECT item.target_id FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
		WHERE operation.id=$2 AND operation.kind='member_early_activation' AND item.target_type='purchase')`, stamp(now), operationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='queued',attempt_started_at=NULL,
		completed_at=NULL,error_code='',updated_at=$1 WHERE operation_id=$2 AND status IN ('failed','pending_review','processing')`, stamp(now), operationID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='queued',attempt_started_at=NULL,completed_at=NULL,
		error_code='',updated_at=$1 WHERE id=$2`, stamp(now), operationID)
	return err
}

// Replace the expired failed artifact with fresh queued work for a recoverable
// live combo command. The receipt and reset phase are reused; no financial action repeats.
func preserveComboControlRecoveryTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT job.id,job.payload FROM maintenance_failed_sync_jobs job JOIN provider_operations operation
		ON operation.id=(CASE ((job.payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (job.payload)::jsonb #>> '{operationId}' END) WHERE job.kind='provider_operation'
		AND operation.status IN ('failed','pending_review') AND `+liveComboControlPredicate,
		stamp(now), stamp(now), stamp(now), stamp(now))
	if err != nil {
		return err
	}
	type recovery struct{ id, payload string }
	var jobs []recovery
	for rows.Next() {
		var job recovery
		if err := rows.Scan(&job.id, &job.payload); err != nil {
			_ = rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := recoverComboControlJobTx(ctx, tx, job.id, now); err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, "provider_operation", job.payload, now, now); err != nil {
			return err
		}
	}
	return nil
}
