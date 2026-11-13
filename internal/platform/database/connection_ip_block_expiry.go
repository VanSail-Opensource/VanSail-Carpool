package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// FinalizeConnectionIPBlockExpiry resolves linked work before scrubbing an expired sensitive row.
func (s *Store) retryAttempt_FinalizeConnectionIPBlockExpiry(ctx context.Context, blockID string, accepted bool, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin connection IP block expiry: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var blockOperation, unblockOperation sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT block_operation_id,unblock_operation_id
		FROM connection_ip_blocks WHERE id=$1`, blockID).Scan(&blockOperation, &unblockOperation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if blockOperation.Valid {
		if err := resolveBlockOperationAtExpiryTx(ctx, tx, blockOperation.String, now.UTC()); err != nil {
			return err
		}
	}
	if unblockOperation.Valid {
		status, code := providerops.StatusPendingReview, "EXPIRY_UNBLOCK_OUTCOME_AMBIGUOUS"
		if accepted {
			status, code = providerops.StatusSucceeded, ""
		}
		if err := resolveLinkedIPOperationTx(ctx, tx, unblockOperation.String,
			connections.UnblockOperationKind, status, code, now.UTC()); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM connection_ip_blocks WHERE id=$1`, blockID); err != nil {
		return fmt.Errorf("scrub expired connection IP block: %w", err)
	}
	return tx.Commit()
}

func resolveBlockOperationAtExpiryTx(ctx context.Context, tx *sql.Tx, operationID string, now time.Time) error {
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM provider_operations WHERE id=$1 AND kind=$2`,
		operationID, connections.BlockOperationKind).Scan(&status); err != nil {
		return err
	}
	switch providerops.Status(status) {
	case providerops.StatusQueued:
		return resolveLinkedIPOperationTx(ctx, tx, operationID, connections.BlockOperationKind,
			providerops.StatusFailed, "BLOCK_WINDOW_EXPIRED", now)
	case providerops.StatusProcessing:
		return resolveLinkedIPOperationTx(ctx, tx, operationID, connections.BlockOperationKind,
			providerops.StatusPendingReview, "BLOCK_EXPIRED_DURING_RECOVERY", now)
	default:
		return nil
	}
}

func resolveLinkedIPOperationTx(ctx context.Context, tx *sql.Tx, operationID, kind string,
	status providerops.Status, code string, now time.Time) error {
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM provider_operations WHERE id=$1 AND kind=$2 FOR UPDATE`, operationID, kind).Scan(&current); err != nil {
		return err
	}
	// A retained terminal command describes its own attempt. Confirmed expiry
	// cleanup is a separate action: preserve that receipt and still scrub the IP.
	if current == "succeeded" || current == "failed" || current == "compensated" {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=$1,error_code=$2,result_json='{}',
		attempt_started_at=COALESCE(attempt_started_at,$3),completed_at=$4,updated_at=$5
		WHERE operation_id=$6 AND target_type='connection_ip_hmac' AND status IN ('queued','processing','pending_review')`,
		status, operationCode(code), stamp(now), stamp(now), stamp(now), operationID)
	if err != nil {
		return fmt.Errorf("resolve expired IP operation item: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrConflict
	}
	result, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status=$1,error_code=$2,result_json='{}',
		attempt_started_at=COALESCE(attempt_started_at,$3),completed_at=$4,updated_at=$5
		WHERE id=$6 AND kind=$7 AND status IN ('queued','processing','pending_review')`, status, operationCode(code),
		stamp(now), stamp(now), stamp(now), operationID, kind)
	if err != nil {
		return fmt.Errorf("resolve expired IP operation: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return ErrConflict
	}
	return nil
}
