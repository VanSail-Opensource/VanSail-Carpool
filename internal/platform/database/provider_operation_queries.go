package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const providerOperationSelect = `SELECT id,actor_user_id,owner_user_id,kind,idempotency_key,request_fingerprint,
	status,attempts,attempt_started_at,provider_reference,error_code,result_json,created_at,updated_at,completed_at
	FROM provider_operations`

// ProviderOperationByID returns internal operation metadata for a worker.
func (s *Store) ProviderOperationByID(ctx context.Context, operationID string) (providerops.Operation, error) {
	return scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, operationID))
}

// ProviderOperationForOwner returns a member-safe receipt scoped to its owner.
func (s *Store) ProviderOperationForOwner(ctx context.Context, operationID, ownerID string) (model.OperationReceipt, error) {
	operation, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+
		` WHERE id=$1 AND owner_user_id=$2`, operationID, ownerID))
	return operation.Receipt, err
}

// ProviderOperationForPrincipal returns a safe receipt to its actor or owner.
func (s *Store) ProviderOperationForPrincipal(ctx context.Context, operationID, userID string) (model.OperationReceipt, error) {
	operation, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+
		` WHERE id=$1 AND (actor_user_id=$2 OR owner_user_id=$3)`, operationID, userID, userID))
	return operation.Receipt, err
}

// ListProviderOperationsForOwner returns recent operation receipts for an aggregate view.
func (s *Store) ListProviderOperationsForOwner(ctx context.Context, ownerID string, limit int) ([]model.OperationReceipt, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, providerOperationSelect+
		` WHERE owner_user_id=$1 ORDER BY `+purchaseTimestampOrderSQL("created_at")+` DESC,id DESC LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list provider operations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]model.OperationReceipt, 0)
	for rows.Next() {
		operation, scanErr := scanProviderOperation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, operation.Receipt)
	}
	return result, rows.Err()
}

func scanProviderOperation(row rowScanner) (providerops.Operation, error) {
	var operation providerops.Operation
	var owner, attempt, completed sql.NullString
	var status, errorCode, created, updated string
	err := row.Scan(&operation.Receipt.ID, &operation.ActorUserID, &owner, &operation.Receipt.Kind,
		&operation.IdempotencyKey, &operation.RequestFingerprint, &status, &operation.Attempts, &attempt,
		&operation.ProviderReference, &errorCode, &operation.ResultJSON, &created, &updated, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return providerops.Operation{}, ErrNotFound
	}
	if err != nil {
		return providerops.Operation{}, fmt.Errorf("scan provider operation: %w", err)
	}
	operation.OwnerUserID = owner.String
	operation.Receipt.Status = status
	if errorCode != "" {
		operation.Receipt.ErrorCode = &errorCode
	}
	if operation.Receipt.Kind == "payment_create" && errorCode == "PAYMENT_CREATE_REJECTED" {
		// Historical provider messages remain internal evidence. An upstream
		// response may echo callback capabilities or signed request payloads.
		message := "Payment provider rejected this checkout request."
		operation.Receipt.ErrorMessage = &message
	}
	if operation.Receipt.CreatedAt, err = parseStamp(created); err != nil {
		return providerops.Operation{}, fmt.Errorf("parse provider operation creation: %w", err)
	}
	if operation.Receipt.UpdatedAt, err = parseStamp(updated); err != nil {
		return providerops.Operation{}, fmt.Errorf("parse provider operation update: %w", err)
	}
	operation.AttemptStartedAt, err = parseOptionalStamp(attempt)
	if err != nil {
		return providerops.Operation{}, err
	}
	operation.Receipt.CompletedAt, err = parseOptionalStamp(completed)
	return operation, err
}

func parseOptionalStamp(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseStamp(value.String)
	if err != nil {
		return nil, fmt.Errorf("parse optional timestamp: %w", err)
	}
	return &parsed, nil
}

// ReplayXNodeCommand retains receipts after a completed physical node deletion.
func (s *Store) ReplayXNodeCommand(ctx context.Context, actor, key, fingerprint string) (model.OperationReceipt, bool, error) {
	op, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` WHERE actor_user_id=$1 AND kind='xnode_command' AND idempotency_key=$2`, actor, key))
	if errors.Is(err, ErrNotFound) {
		return model.OperationReceipt{}, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if op.RequestFingerprint != fingerprint {
		return op.Receipt, false, ErrConflict
	}
	return op.Receipt, true, nil
}
