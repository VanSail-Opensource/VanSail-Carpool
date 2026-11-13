package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// AdminOperationResolutionInput is one idempotent audited ambiguity decision.
type AdminOperationResolutionInput struct {
	ActorUserID, OperationID, IdempotencyKey, RequestFingerprint string
	Resolution, Reason                                           string
}

// ErrOperationReconciliationRequired denies receipt repair without matching
// retained domain evidence. It never authorizes a provider or financial override.
var ErrOperationReconciliationRequired = fmt.Errorf("%w: operation still requires domain reconciliation", ErrConflict)

// ResolveAdminOperation repairs a supported parent receipt from terminal item
// evidence, without changing provider state, items, holds or notification gates.
func (s *Store) retryAttempt_ResolveAdminOperation(ctx context.Context, input AdminOperationResolutionInput,
	now time.Time) (model.OperationReceipt, error) {
	if strings.TrimSpace(input.OperationID) == "" || strings.TrimSpace(input.IdempotencyKey) == "" || len(input.IdempotencyKey) > 128 ||
		len(input.RequestFingerprint) < 16 || len(input.RequestFingerprint) > 128 || len(strings.TrimSpace(input.Reason)) < 3 || len(input.Reason) > 500 {
		return model.OperationReceipt{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = catalogAdminActorTx(ctx, tx, input.ActorUserID); err != nil {
		return model.OperationReceipt{}, err
	}
	if receipt, replayed, replayErr := resolvedOperationReplay(ctx, tx, input); replayErr != nil || replayed {
		return commitResolvedReplay(tx, receipt, replayErr)
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1 FOR UPDATE`, input.OperationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	before := operation.Receipt.Status
	if (before != "pending_review" && before != "partial") || before == input.Resolution {
		return model.OperationReceipt{}, ErrConflict
	}
	if err = adminResolutionEvidenceTx(ctx, tx, operation.Receipt.Kind, input.OperationID, input.Resolution); err != nil {
		return model.OperationReceipt{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status=$1,updated_at=$2,completed_at=$3,error_code=CASE WHEN $1='succeeded' THEN '' ELSE error_code END
		WHERE id=$4 AND status=$5`, input.Resolution, stamp(now), stamp(now), input.OperationID, before)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.OperationReceipt{}, rowsErr
		}
		return model.OperationReceipt{}, ErrConflict
	}
	updated, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, input.OperationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	replayID, err := ids.New()
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if err := insertResolutionReplay(ctx, tx, replayID, input, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := insertResolutionAudit(ctx, tx, replayID, input, before, updated.Receipt, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return updated.Receipt, nil
}
