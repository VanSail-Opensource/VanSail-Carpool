package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// Called only after normalization, exact replay lookup, and the admission
// locks for a fresh command or a validated stored-ban expiry continuation.
func insertProviderOperationTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput, now time.Time, completed bool) (providerops.Operation, bool, error) {
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
		request_fingerprint,status,created_at,updated_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, operationID,
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
			status,created_at,updated_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, operationID, item.Key, item.TargetType,
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
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, operationID))
	if err != nil {
		return providerops.Operation{}, false, err
	}
	return operation, false, nil
}
