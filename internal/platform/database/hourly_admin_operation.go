package database

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CreateAdminHourlyDisableOperation admits only the trusted administrative
// branch of the shared hourly disable kind. Execution resumes committed intent
// without consulting the actor's later role; fresh admission locks that role.
func (s *Store) retryAttempt_CreateAdminHourlyDisableOperation(ctx context.Context, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if input.Kind != "xnode_hourly_disable" || input.OwnerUserID == "" || input.SealedTarget == "" {
		return providerops.Operation{}, false, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer tx.Rollback()
	now = now.UTC()
	if operation, replayed, err := replayProviderOperationTx(ctx, tx, input, now); err != nil || replayed {
		if err != nil {
			return providerops.Operation{}, false, err
		}
		return operation, true, tx.Commit()
	}
	if err = catalogAdminActorTx(ctx, tx, input.ActorUserID); err != nil {
		return providerops.Operation{}, false, err
	}
	if err = lockProviderOperationAccountsTx(ctx, tx, input.ActorUserID, input.OwnerUserID); err != nil {
		return providerops.Operation{}, false, err
	}
	operation, _, err := insertProviderOperationTx(ctx, tx, input, now, false)
	if err != nil {
		if !errors.Is(err, ErrConflict) {
			return providerops.Operation{}, false, err
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return providerops.Operation{}, false, errors.Join(err, rollbackErr)
		}
		return s.replayAdminHourlyDisableInsertConflict(ctx, input, now, err)
	}
	if err = tx.Commit(); err != nil {
		return providerops.Operation{}, false, err
	}
	return operation, false, nil
}

func (s *Store) replayAdminHourlyDisableInsertConflict(ctx context.Context, input providerops.CreateInput, now time.Time, cause error) (providerops.Operation, bool, error) {
	// The earlier serializable snapshot can miss a newly committed command.
	// Consult exact evidence only after the losing transaction has rolled back.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, errors.Join(cause, err)
	}
	defer tx.Rollback()
	operation, replayed, err := replayProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, false, errors.Join(cause, err)
	}
	if !replayed {
		return providerops.Operation{}, false, cause
	}
	if err = tx.Commit(); err != nil {
		return providerops.Operation{}, false, errors.Join(cause, err)
	}
	return operation, true, nil
}
