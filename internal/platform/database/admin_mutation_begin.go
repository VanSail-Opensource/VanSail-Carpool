package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CreateAdminMutation commits the queue intent and its privileged audit together.
func (s *Store) retryAttempt_CreateAdminMutation(ctx context.Context, input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil || len(input.Items) != 1 || input.Items[0].Key != "target" {
		return providerops.Operation{}, false, ErrConflict
	}
	item := input.Items[0]
	switch input.Kind {
	case providerops.KindPaymentRefund:
		if item.TargetType != "payment_order" || input.OwnerUserID == "" {
			return providerops.Operation{}, false, ErrConflict
		}
	case providerops.KindOutboxRetry:
		if item.TargetType != "outbox_job" || input.OwnerUserID != input.ActorUserID {
			return providerops.Operation{}, false, ErrConflict
		}
	case providerops.KindAdminMaintenance:
		if item.TargetType != "maintenance" || item.TargetID != "retention" || input.OwnerUserID != input.ActorUserID {
			return providerops.Operation{}, false, ErrConflict
		}
	default:
		return providerops.Operation{}, false, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if operation.OwnerUserID != input.OwnerUserID {
		return providerops.Operation{}, false, ErrConflict
	}
	if !replayed {
		if input.Kind == providerops.KindPaymentRefund {
			var owner string
			err := tx.QueryRowContext(ctx, `SELECT user_id FROM payment_orders WHERE id=$1`, item.TargetID).Scan(&owner)
			if errors.Is(err, sql.ErrNoRows) {
				return providerops.Operation{}, false, ErrNotFound
			}
			if err != nil {
				return providerops.Operation{}, false, err
			}
			if owner != input.OwnerUserID {
				return providerops.Operation{}, false, ErrConflict
			}
		}
		auditID, err := ids.New()
		if err != nil {
			return providerops.Operation{}, false, err
		}
		detail, err := json.Marshal(map[string]string{"operationId": operation.Receipt.ID})
		if err != nil {
			return providerops.Operation{}, false, err
		}
		if err := insertAuditTx(ctx, tx, auditID, &input.ActorUserID, input.Kind+".queue", item.TargetType, item.TargetID, string(detail), now); err != nil {
			return providerops.Operation{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, err
	}
	return operation, replayed, nil
}
