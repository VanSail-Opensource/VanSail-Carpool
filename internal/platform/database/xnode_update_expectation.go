package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// PrepareXNodeUpdate stores immutable intent in the existing operation receipt.
// An attempt that already started without intent cannot invent recovery evidence.
func (s *Store) retryAttempt_PrepareXNodeUpdate(ctx context.Context, id string, expected xnode.UpdateExpectation, now time.Time) (providerops.Operation, error) {
	if !expected.Valid() || (expected.Command == "update.install" && expected.TransactionID != id) {
		return providerops.Operation{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return operation, err
	}
	if operation.Receipt.Kind != xnode.OperationKind {
		return operation, errors.Join(ErrConflict, xnode.ErrUpdateEvidence)
	}
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return operation, tx.Commit()
	}
	if existing, ok := xnode.UpdateExpectationFromResult(operation.ResultJSON); ok {
		if existing.Command != expected.Command {
			return operation, errors.Join(ErrConflict, xnode.ErrUpdateEvidence)
		}
		return operation, tx.Commit()
	}
	if operation.Attempts != 0 || (operation.ResultJSON != "" && operation.ResultJSON != "{}") {
		return operation, errors.Join(ErrConflict, xnode.ErrUpdateEvidence)
	}
	raw, err := json.Marshal(map[string]any{"updateExpectation": expected})
	if err != nil {
		return operation, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operations SET result_json=$1,updated_at=$2 WHERE id=$3`, string(raw), stamp(now.UTC()), id); err != nil {
		return operation, err
	}
	operation.ResultJSON = string(raw)
	operation.Receipt.UpdatedAt = now.UTC()
	return operation, tx.Commit()
}
