package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Both period admission and retained-period restoration use the same locked
// target and financial evidence. No external work runs in this transaction.
func validatePurchasePeriodTargetTx(ctx context.Context, tx *sql.Tx, expected model.Purchase, now time.Time) error {
	if expected.ID == "" || expected.UserID == "" || expected.UpdatedAt.IsZero() || now.IsZero() {
		return xnode.ErrConflict
	}
	current, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2 FOR UPDATE OF purchases`, expected.ID, expected.UserID))
	if errors.Is(err, ErrNotFound) {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	current.SquadUUIDs, err = purchaseSquadsFrom(ctx, tx, current.ID)
	if err != nil {
		return err
	}
	if !current.ValidUntil.Equal(expected.ValidUntil) {
		// EffectivePurchase may retain the predecessor's accounting across an
		// already-paid queued successor. Only that exact durable lineage can
		// extend this projection; it never edits the original purchase.
		linked, err := purchasePeriodContinuityTx(ctx, tx, current, expected.ValidUntil)
		if err != nil {
			return err
		}
		if !linked {
			return xnode.ErrConflict
		}
		current.ValidUntil = expected.ValidUntil
	}
	if !samePurchaseActivationMaterial(current, expected) || current.Status != expected.Status || !current.UpdatedAt.Equal(expected.UpdatedAt) ||
		now.Before(current.ValidFrom) || !now.Before(current.ValidUntil) ||
		(current.Status != "active" && current.Status != "activating" && current.Status != "failed") {
		return xnode.ErrConflict
	}
	refunded, err := entitlementRefundedTx(ctx, tx, current)
	if err != nil {
		return err
	}
	if refunded {
		return xnode.ErrConflict
	}
	// A retained failed/old term cannot replace another currently paid target.
	rows, err := tx.QueryContext(ctx, `SELECT valid_from,valid_until FROM purchases WHERE user_id=$1 AND id<>$2 AND status IN ('active','activating')`, current.UserID, current.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var fromText, untilText string
		if err := rows.Scan(&fromText, &untilText); err != nil {
			return err
		}
		from, err := parseStamp(fromText)
		if err != nil {
			return err
		}
		until, err := parseStamp(untilText)
		if err != nil {
			return err
		}
		if !now.Before(from) && now.Before(until) {
			return xnode.ErrConflict
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var blocked bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_operations operation JOIN provider_operation_items item ON item.operation_id=operation.id
		WHERE operation.kind IN ('purchase_refund','purchase_traffic_reset') AND operation.status IN ('queued','processing','pending_review','partial')
		AND item.target_type='purchase' AND item.target_id=$1)
		OR EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=$2 AND restored_at IS NULL)`, current.ID, current.UserID).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return xnode.ErrAccessOperationPending
	}
	return nil
}

func purchasePeriodContinuityTx(ctx context.Context, tx *sql.Tx, current model.Purchase, until time.Time) (bool, error) {
	if (current.Status != "active" && current.Status != "activating") || !until.After(current.ValidUntil) {
		return false, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT valid_from,valid_until FROM purchases WHERE user_id=$1 AND status='queued' FOR SHARE`, current.UserID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	linked := false
	for rows.Next() {
		var fromText, untilText string
		if err := rows.Scan(&fromText, &untilText); err != nil {
			return false, err
		}
		from, err := parseStamp(fromText)
		if err != nil {
			return false, err
		}
		nextUntil, err := parseStamp(untilText)
		if err != nil {
			return false, err
		}
		linked = linked || from.Equal(current.ValidUntil) && nextUntil.Equal(until)
	}
	return linked, rows.Err()
}
