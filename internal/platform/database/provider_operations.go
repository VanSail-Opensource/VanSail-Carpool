package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CreateProviderOperation persists an idempotent command and queues it atomically.
func (s *Store) retryAttempt_CreateProviderOperation(ctx context.Context, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	now = now.UTC()

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
	input, err := providerops.NormalizeCreate(input)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if existing, replayed, err := replayProviderOperationTx(ctx, tx, input, now); err != nil || replayed {
		return existing, replayed, err
	}
	if err := providerOperationAdminActorTx(ctx, tx, input); err != nil {
		return providerops.Operation{}, false, err
	}
	if err := lockProviderOperationAccountsTx(ctx, tx, input.ActorUserID, input.OwnerUserID); err != nil {
		return providerops.Operation{}, false, err
	}
	return insertProviderOperationTx(ctx, tx, input, now, completed)
}

// Recover the immutable command before admission checks against mutable live
// state. Items may be absent when the original target set is no longer current.
func replayProviderOperationTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	existing, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+
		` WHERE actor_user_id=$1 AND kind=$2 AND idempotency_key=$3`, input.ActorUserID, input.Kind, input.IdempotencyKey))
	if errors.Is(err, ErrNotFound) {
		return providerops.Operation{}, false, nil
	}
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if existing.RequestFingerprint != input.RequestFingerprint {
		return providerops.Operation{}, false, ErrConflict
	}
	if err := insertOperationReplayTx(ctx, tx, existing.Receipt.ID, input, now); err != nil {
		return providerops.Operation{}, false, err
	}
	return existing, true, nil
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
	_, err = tx.ExecContext(ctx, `INSERT   INTO provider_operation_replays(id,operation_id,actor_user_id,
		idempotency_key,request_fingerprint,created_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING `, replayID, operationID,
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
