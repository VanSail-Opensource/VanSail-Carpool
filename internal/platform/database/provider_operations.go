package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CreateProviderOperation persists an idempotent command and queues it atomically.
func (s *Store) retryAttempt_CreateProviderOperation(ctx context.Context, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	now = now.UTC()
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, fmt.Errorf("begin provider operation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("commit provider operation: %w", err)
	}
	return operation, replayed, nil
}

// createProviderOperationTx lets local mutations and provider queueing share one transaction.
func createProviderOperationTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	return createProviderOperationStateTx(ctx, tx, input, now, false)
}

// Local preference receipts preserve replay compatibility without an outbox job.
func createProviderOperationStateTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput, now time.Time, completed bool) (providerops.Operation, bool, error) {
	var unavailable bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE (id=? OR id=?) AND account_status<>'active')`, input.ActorUserID, input.OwnerUserID).Scan(&unavailable); err != nil {
		return providerops.Operation{}, false, err
	}
	if unavailable {
		return providerops.Operation{}, false, ErrAccountUnavailable
	}
	input, err := providerops.NormalizeCreate(input)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	existing, loadErr := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+
		` WHERE actor_user_id=? AND kind=? AND idempotency_key=?`, input.ActorUserID, input.Kind, input.IdempotencyKey))
	if loadErr == nil {
		if existing.RequestFingerprint != input.RequestFingerprint {
			return providerops.Operation{}, false, ErrConflict
		}
		if err := insertOperationReplayTx(ctx, tx, existing.Receipt.ID, input, now); err != nil {
			return providerops.Operation{}, false, err
		}
		return existing, true, nil
	}
	if !errors.Is(loadErr, ErrNotFound) {
		return providerops.Operation{}, false, loadErr
	}
	operationID, err := ids.New()
	if err != nil {
		return providerops.Operation{}, false, err
	}
	status := "queued"
	var finished any
	if completed {
		status, finished = "succeeded", stamp(now)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO provider_operations(id,actor_user_id,owner_user_id,kind,idempotency_key,
		request_fingerprint,status,created_at,updated_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, operationID,
		input.ActorUserID, nullIfEmpty(input.OwnerUserID), input.Kind, input.IdempotencyKey,
		input.RequestFingerprint, status, stamp(now), stamp(now), finished)
	if err != nil {
		if isUniqueViolation(err) {
			return providerops.Operation{}, false, ErrConflict
		}
		return providerops.Operation{}, false, fmt.Errorf("insert provider operation: %w", err)
	}
	for _, item := range input.Items {
		_, err := tx.ExecContext(ctx, `INSERT INTO provider_operation_items(operation_id,item_key,target_type,target_id,
			status,created_at,updated_at,completed_at) VALUES(?,?,?,?,?,?,?,?)`, operationID, item.Key, item.TargetType,
			item.TargetID, status, stamp(now), stamp(now), finished)
		if err != nil {
			return providerops.Operation{}, false, fmt.Errorf("insert provider operation item: %w", err)
		}
	}
	if !completed {
		job := map[string]string{"operationId": operationID}
		if input.SealedTarget != "" {
			job["sealedTarget"] = input.SealedTarget
		}
		payload, err := json.Marshal(job)
		if err != nil {
			return providerops.Operation{}, false, fmt.Errorf("encode provider operation job: %w", err)
		}
		if err := insertOutboxTx(ctx, tx, "provider_operation", string(payload), now, now); err != nil {
			return providerops.Operation{}, false, err
		}
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, operationID))
	if err != nil {
		return providerops.Operation{}, false, err
	}
	return operation, false, nil
}

// CreateOrReplayProviderOperation is an explicit alias for command services.
func (s *Store) CreateOrReplayProviderOperation(ctx context.Context, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	return s.CreateProviderOperation(ctx, input, now)
}

func insertOperationReplayTx(ctx context.Context, tx *sql.Tx, operationID string, input providerops.CreateInput, now time.Time) error {
	replayID, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO provider_operation_replays(id,operation_id,actor_user_id,
		idempotency_key,request_fingerprint,created_at) VALUES(?,?,?,?,?,?)`, replayID, operationID,
		input.ActorUserID, input.IdempotencyKey, input.RequestFingerprint, stamp(now))
	if err != nil {
		return fmt.Errorf("record provider operation replay: %w", err)
	}
	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
