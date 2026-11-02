package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CompleteConnectionIPBlockOperation atomically closes a receipt and transitions its sensitive row.
func (s *Store) retryAttempt_CompleteConnectionIPBlockOperation(ctx context.Context, blockID, operationID, itemKey string,
	completion connections.BlockOperationCompletion, now time.Time) error {
	if !validBlockCompletion(completion) {
		return ErrConflict
	}
	resultJSON, err := providerops.ResultObject(completion.Operation.ResultJSON)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin connection IP block completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var expiryJobID string
	if err := tx.QueryRowContext(ctx, `SELECT expiry_job_id FROM connection_ip_blocks WHERE id=$1
		AND (block_operation_id=$2 OR unblock_operation_id=$3)`, blockID, operationID, operationID).Scan(&expiryJobID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connections.ErrIPBlockNotFound
		}
		return err
	}
	if err := completeIPBlockReceiptTx(ctx, tx, operationID, itemKey, completion, resultJSON, now.UTC()); err != nil {
		return err
	}
	if completion.RemoveBlock {
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=$1 AND status<>'processing'`, expiryJobID); err != nil {
			return fmt.Errorf("cancel IP block expiry: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM connection_ip_blocks WHERE id=$1`, blockID); err != nil {
			return fmt.Errorf("delete connection IP block: %w", err)
		}
	} else {
		clearUnblock := 0
		if completion.ClearUnblock {
			clearUnblock = 1
		}
		result, err := tx.ExecContext(ctx, `UPDATE connection_ip_blocks SET status=$1,
			unblock_operation_id=CASE WHEN $2=1 THEN NULL ELSE unblock_operation_id END,updated_at=$3 WHERE id=$4`,
			completion.BlockStatus, clearUnblock, stamp(now), blockID)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return ErrConflict
		}
	}
	return tx.Commit()
}

func completeIPBlockReceiptTx(ctx context.Context, tx *sql.Tx, operationID, itemKey string,
	completion connections.BlockOperationCompletion, resultJSON string, now time.Time) error {
	result, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=$1,provider_reference=$2,error_code=$3,
		result_json=$4,attempt_started_at=COALESCE(attempt_started_at,$5),completed_at=$6,updated_at=$7
		WHERE operation_id=$8 AND item_key=$9 AND status IN ('queued','processing')`, completion.ItemStatus,
		strings.TrimSpace(completion.Operation.ProviderReference), operationCode(completion.Operation.ErrorCode), resultJSON,
		stamp(now), stamp(now), stamp(now), operationID, itemKey)
	if err != nil {
		return fmt.Errorf("complete connection IP block item: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrConflict
	}
	result, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status=$1,provider_reference=$2,error_code=$3,
		result_json=$4,completed_at=$5,updated_at=$6 WHERE id=$7 AND status='processing' AND attempt_started_at IS NOT NULL`,
		completion.Operation.Status, strings.TrimSpace(completion.Operation.ProviderReference),
		operationCode(completion.Operation.ErrorCode), resultJSON, stamp(now), stamp(now), operationID)
	if err != nil {
		return fmt.Errorf("complete connection IP block operation: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrConflict
	}
	return nil
}

func validBlockCompletion(completion connections.BlockOperationCompletion) bool {
	itemTerminal := completion.ItemStatus == providerops.StatusSucceeded || completion.ItemStatus == providerops.StatusFailed ||
		completion.ItemStatus == providerops.StatusPendingReview || completion.ItemStatus == providerops.StatusCompensated
	validStatus := completion.BlockStatus == connections.BlockStatusBlocking || completion.BlockStatus == connections.BlockStatusActive ||
		completion.BlockStatus == connections.BlockStatusUnblocking || completion.BlockStatus == connections.BlockStatusPendingReview
	return providerops.Terminal(completion.Operation.Status) && itemTerminal && (completion.RemoveBlock || validStatus) &&
		!(completion.RemoveBlock && completion.ClearUnblock)
}
