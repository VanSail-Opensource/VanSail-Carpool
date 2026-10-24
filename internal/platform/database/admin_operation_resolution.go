package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// AdminOperationResolutionInput is one idempotent audited ambiguity decision.
type AdminOperationResolutionInput struct {
	ActorUserID, OperationID, IdempotencyKey, RequestFingerprint string
	Resolution, Reason                                           string
}

// ResolveAdminOperation records a human decision without another provider attempt.
func (s *Store) retryAttempt_ResolveAdminOperation(ctx context.Context, input AdminOperationResolutionInput,
	now time.Time) (model.OperationReceipt, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if receipt, replayed, replayErr := resolvedOperationReplay(ctx, tx, input); replayErr != nil || replayed {
		return commitResolvedReplay(tx, receipt, replayErr)
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, input.OperationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	before := operation.Receipt.Status
	if (before != "pending_review" && before != "partial") || before == input.Resolution {
		return model.OperationReceipt{}, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status=?,updated_at=?,completed_at=?
		WHERE id=? AND status=?`, input.Resolution, stamp(now), stamp(now), input.OperationID, before)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.OperationReceipt{}, rowsErr
		}
		return model.OperationReceipt{}, ErrConflict
	}
	if err := insertResolutionReplay(ctx, tx, input, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := insertResolutionAudit(ctx, tx, input, before, now); err != nil {
		return model.OperationReceipt{}, err
	}
	updated, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, input.OperationID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return updated.Receipt, nil
}

func resolvedOperationReplay(ctx context.Context, tx *sql.Tx, input AdminOperationResolutionInput) (model.OperationReceipt, bool, error) {
	var fingerprint string
	err := tx.QueryRowContext(ctx, `SELECT request_fingerprint FROM provider_operation_replays
		WHERE operation_id=? AND actor_user_id=? AND idempotency_key=? LIMIT 1`, input.OperationID,
		input.ActorUserID, input.IdempotencyKey).Scan(&fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return model.OperationReceipt{}, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if fingerprint != input.RequestFingerprint {
		return model.OperationReceipt{}, false, ErrConflict
	}
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, input.OperationID))
	return operation.Receipt, err == nil, err
}

func commitResolvedReplay(tx *sql.Tx, receipt model.OperationReceipt, err error) (model.OperationReceipt, error) {
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return receipt, nil
}

func insertResolutionReplay(ctx context.Context, tx *sql.Tx, input AdminOperationResolutionInput, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO provider_operation_replays(id,operation_id,actor_user_id,
		idempotency_key,request_fingerprint,created_at) VALUES(?,?,?,?,?,?)`, id, input.OperationID,
		input.ActorUserID, input.IdempotencyKey, input.RequestFingerprint, stamp(now))
	return err
}

func insertResolutionAudit(ctx context.Context, tx *sql.Tx, input AdminOperationResolutionInput, before string, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]string{"reason": input.Reason, "before": before, "after": input.Resolution})
	if err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, id, &input.ActorUserID, "provider_operation.resolve",
		"provider_operation", input.OperationID, string(detail), now); err != nil {
		return fmt.Errorf("audit operation resolution: %w", err)
	}
	return nil
}
