package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// BeginProviderOperationAttempt durably marks the attempt before provider mutation.
func (s *Store) retryAttempt_BeginProviderOperationAttempt(ctx context.Context, operationID string, now time.Time) (providerops.Operation, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, fmt.Errorf("begin provider attempt: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status='processing',attempts=attempts+1,
		attempt_started_at=$1,updated_at=$2 WHERE id=$3 AND (status='queued' OR (status='processing' AND kind IN('xnode_command','xnode_routing','xnode_route_speedtest','xnode_diy','xnode_ddns_update')))`, stamp(now), stamp(now), operationID)
	if err != nil {
		return providerops.Operation{}, fmt.Errorf("mark provider attempt: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return providerops.Operation{}, fmt.Errorf("inspect provider attempt: %w", rowsErr)
	} else if affected != 1 {
		return providerops.Operation{}, ErrConflict
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, operationID))
	if err != nil {
		return providerops.Operation{}, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, fmt.Errorf("commit provider attempt: %w", err)
	}
	return operation, nil
}

// CompleteProviderOperation persists a sanitized terminal result.
func (s *Store) CompleteProviderOperation(ctx context.Context, operationID string, completion providerops.Completion, now time.Time) (providerops.Operation, error) {
	return s.completeProviderOperation(ctx, operationID, nil, completion, now)
}

// CompleteXNodeProviderOperation fences recovered asynchronous attempts. Node
// commands are idempotent by receipt ID; older callbacks cannot overwrite replay.
func (s *Store) CompleteXNodeProviderOperation(ctx context.Context, operationID string, attempt int, completion providerops.Completion, now time.Time) (providerops.Operation, error) {
	return s.completeProviderOperation(ctx, operationID, &attempt, completion, now)
}

func (s *Store) retryAttempt_completeProviderOperation(ctx context.Context, operationID string, attempt *int, completion providerops.Completion, now time.Time) (providerops.Operation, error) {
	if !providerops.Terminal(completion.Status) {
		return providerops.Operation{}, ErrConflict
	}
	resultJSON, err := providerops.ResultObject(completion.ResultJSON)
	if err != nil {
		return providerops.Operation{}, err
	}
	completion.ProviderReference = strings.TrimSpace(completion.ProviderReference)
	completion.ErrorCode = operationCode(completion.ErrorCode)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, fmt.Errorf("begin provider completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var kind, existingResult string
	if err := tx.QueryRowContext(ctx, `SELECT kind,result_json FROM provider_operations WHERE id=$1 FOR UPDATE`, operationID).Scan(&kind, &existingResult); err != nil {
		return providerops.Operation{}, fmt.Errorf("lock provider completion: %w", err)
	}
	if kind == "xnode_diy" {
		resultJSON, err = mergeProviderResult(existingResult, resultJSON)
		if err != nil {
			return providerops.Operation{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status=$1,provider_reference=$2,error_code=$3,
		result_json=$4,completed_at=$5,updated_at=$6 WHERE id=$7 AND ((status='processing' AND attempt_started_at IS NOT NULL)
		OR (status='queued' AND $8='failed')) AND (CAST($9 AS BIGINT) IS NULL OR attempts=$10)`, completion.Status, completion.ProviderReference, completion.ErrorCode,
		resultJSON, stamp(now), stamp(now), operationID, completion.Status, attempt, attempt)
	if err != nil {
		return providerops.Operation{}, fmt.Errorf("complete provider operation: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return providerops.Operation{}, fmt.Errorf("inspect provider completion: %w", rowsErr)
	} else if affected != 1 {
		return providerops.Operation{}, ErrConflict
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, operationID))
	if err != nil {
		return providerops.Operation{}, err
	}
	if completion.Status == providerops.StatusSucceeded && operation.Receipt.Kind == purchaseops.OperationResetKind {
		if err := s.insertManualResetNoticeTx(ctx, tx, operation, false, "", now); err != nil {
			return providerops.Operation{}, err
		}
	}
	if operation.Receipt.Kind == providerops.KindTelegramPMRelay {
		// Unsent repair waiters remain resumable until the bounded payload expires.
		if _, err := tx.ExecContext(ctx, `UPDATE telegram_pm_payloads SET encrypted_payload='' WHERE operation_id=$1 AND
			($2<>'pending_review' OR NOT EXISTS(SELECT 1 FROM provider_operation_items WHERE operation_id=$3 AND item_key='relay' AND status='queued'))`, operationID, completion.Status, operationID); err != nil {
			return providerops.Operation{}, err
		}
	}
	if completion.Status == providerops.StatusFailed && operation.Receipt.Kind == purchaseops.OperationRefundKind {
		if err := s.insertMemberRefundFailedNoticeTx(ctx, tx, operation, completion.ErrorCode, now); err != nil {
			return providerops.Operation{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, fmt.Errorf("commit provider completion: %w", err)
	}
	return operation, nil
}

func operationCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) > 80 {
		return value[:80]
	}
	return value
}
