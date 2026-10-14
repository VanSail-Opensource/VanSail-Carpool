package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CompleteOutboxRetryOperation reactivates one job and completes its receipt atomically.
func (s *Store) CompleteOutboxRetryOperation(ctx context.Context, operationID, itemKey, jobID string,
	now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var targetID string
	err = tx.QueryRowContext(ctx, `SELECT item.target_id FROM provider_operation_items item
		JOIN provider_operations operation ON operation.id=item.operation_id
		WHERE operation.id=? AND operation.kind=? AND operation.status='processing'
		AND item.item_key=? AND item.target_type='outbox_job' AND item.status='processing'`,
		operationID, providerops.KindOutboxRetry, itemKey).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) || targetID != jobID {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	var kind, status string
	if err := tx.QueryRowContext(ctx, `SELECT kind,status FROM outbox_jobs WHERE id=?`, jobID).Scan(&kind, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status == "failed" {
		if err := reactivateOutboxJobTx(ctx, tx, jobID, kind, now.UTC()); err != nil {
			return err
		}
	} else if status != "pending" && status != "processing" {
		return ErrConflict
	}
	stampNow := stamp(now.UTC())
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',error_code='',
		result_json='{}',completed_at=?,updated_at=? WHERE operation_id=? AND item_key=? AND status='processing'`,
		stampNow, stampNow, operationID, itemKey); err != nil {
		return fmt.Errorf("complete outbox retry item: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded',error_code='',
		result_json='{}',completed_at=?,updated_at=? WHERE id=? AND status='processing'`,
		stampNow, stampNow, operationID)
	if err != nil {
		return fmt.Errorf("complete outbox retry operation: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return rowsErr
		}
		return ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit outbox retry operation: %w", err)
	}
	return nil
}

func reactivateOutboxJobTx(ctx context.Context, tx *sql.Tx, jobID, kind string, now time.Time) error {
	if kind == "provider_operation" {
		if err := recoverComboControlJobTx(ctx, tx, jobID, now); err != nil {
			return err
		}
	}
	if kind == "questionnaire_settlement" {
		result, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET status='queued',last_error='',updated_at=?
			WHERE status='failed' AND id=(SELECT json_extract(payload,'$.importId') FROM outbox_jobs WHERE id=?)`,
			stamp(now), jobID)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return ErrConflict
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=0,last_error='',
		available_at=?,updated_at=? WHERE id=? AND status='failed'`, stamp(now), stamp(now), jobID)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrConflict
	}
	return nil
}
