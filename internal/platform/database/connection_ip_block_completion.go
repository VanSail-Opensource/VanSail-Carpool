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
func (s *Store) CompleteConnectionIPBlockOperation(ctx context.Context, blockID, operationID, itemKey string,
	completion connections.BlockOperationCompletion, now time.Time) error {
	if !validBlockCompletion(completion) {
		return ErrConflict
	}
	resultJSON, err := providerops.ResultObject(completion.Operation.ResultJSON)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin connection IP block completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var expiryJobID string
	if err := tx.QueryRowContext(ctx, `SELECT expiry_job_id FROM connection_ip_blocks WHERE id=?
		AND (block_operation_id=? OR unblock_operation_id=?)`, blockID, operationID, operationID).Scan(&expiryJobID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connections.ErrIPBlockNotFound
		}
		return err
	}
	if err := completeIPBlockReceiptTx(ctx, tx, operationID, itemKey, completion, resultJSON, now.UTC()); err != nil {
		return err
	}
	if completion.RemoveBlock {
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=? AND status<>'processing'`, expiryJobID); err != nil {
			return fmt.Errorf("cancel IP block expiry: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM connection_ip_blocks WHERE id=?`, blockID); err != nil {
			return fmt.Errorf("delete connection IP block: %w", err)
		}
	} else {
		clearUnblock := 0
		if completion.ClearUnblock {
			clearUnblock = 1
		}
		result, err := tx.ExecContext(ctx, `UPDATE connection_ip_blocks SET status=?,
			unblock_operation_id=CASE WHEN ?=1 THEN NULL ELSE unblock_operation_id END,updated_at=? WHERE id=?`,
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
	result, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=?,provider_reference=?,error_code=?,
		result_json=?,attempt_started_at=COALESCE(attempt_started_at,?),completed_at=?,updated_at=?
		WHERE operation_id=? AND item_key=? AND status IN ('queued','processing')`, completion.ItemStatus,
		strings.TrimSpace(completion.Operation.ProviderReference), operationCode(completion.Operation.ErrorCode), resultJSON,
		stamp(now), stamp(now), stamp(now), operationID, itemKey)
	if err != nil {
		return fmt.Errorf("complete connection IP block item: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrConflict
	}
	result, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status=?,provider_reference=?,error_code=?,
		result_json=?,completed_at=?,updated_at=? WHERE id=? AND status='processing' AND attempt_started_at IS NOT NULL`,
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
