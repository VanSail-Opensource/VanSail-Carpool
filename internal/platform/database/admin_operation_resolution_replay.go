package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func resolvedOperationReplay(ctx context.Context, tx *sql.Tx, input AdminOperationResolutionInput) (model.OperationReceipt, bool, error) {
	var fingerprint, created string
	var detail sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT r.request_fingerprint,r.created_at,a.detail FROM provider_operation_replays r
		LEFT JOIN audit_events a ON a.id=r.id AND a.actor_user_id=r.actor_user_id AND a.target_id=r.operation_id
		 AND a.action='provider_operation.resolve' AND a.target_type='provider_operation'
		WHERE r.operation_id=$1 AND r.actor_user_id=$2 AND r.idempotency_key=$3 LIMIT 1`, input.OperationID,
		input.ActorUserID, input.IdempotencyKey).Scan(&fingerprint, &created, &detail)
	if errors.Is(err, sql.ErrNoRows) {
		return model.OperationReceipt{}, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if fingerprint != input.RequestFingerprint {
		return model.OperationReceipt{}, false, ErrConflict
	}
	if detail.Valid {
		var recorded struct {
			Receipt *model.OperationReceipt `json:"receipt"`
		}
		if json.Unmarshal([]byte(detail.String), &recorded) != nil || recorded.Receipt == nil ||
			recorded.Receipt.ID != input.OperationID || recorded.Receipt.Status != input.Resolution || recorded.Receipt.CompletedAt == nil {
			return model.OperationReceipt{}, false, ErrConflict
		}
		return *recorded.Receipt, true, nil
	}
	// Older resolutions did not retain a result snapshot. Their unchanged
	// receipt can still replay, but later mutable state is never substituted for
	// that historical decision. Existing evidence remains untouched.
	operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1`, input.OperationID))
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	at, err := parseStamp(created)
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if operation.Receipt.Status != input.Resolution || !operation.Receipt.UpdatedAt.Equal(at) ||
		operation.Receipt.CompletedAt == nil || !operation.Receipt.CompletedAt.Equal(at) {
		return model.OperationReceipt{}, false, ErrConflict
	}
	return operation.Receipt, true, nil
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

func insertResolutionReplay(ctx context.Context, tx *sql.Tx, id string, input AdminOperationResolutionInput, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO provider_operation_replays(id,operation_id,actor_user_id,
		idempotency_key,request_fingerprint,created_at) VALUES($1,$2,$3,$4,$5,$6)`, id, input.OperationID,
		input.ActorUserID, input.IdempotencyKey, input.RequestFingerprint, stamp(now))
	return err
}

func insertResolutionAudit(ctx context.Context, tx *sql.Tx, id string, input AdminOperationResolutionInput, before string, receipt model.OperationReceipt, now time.Time) error {
	detail, err := json.Marshal(map[string]any{"reason": input.Reason, "before": before, "after": input.Resolution, "receipt": receipt})
	if err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, id, &input.ActorUserID, "provider_operation.resolve",
		"provider_operation", input.OperationID, string(detail), now); err != nil {
		return fmt.Errorf("audit operation resolution: %w", err)
	}
	return nil
}
