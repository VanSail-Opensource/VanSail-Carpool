package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// BeginEarlyActivation forfeits and rebases terms without altering balances or prices.
func (s *Store) retryAttempt_BeginEarlyActivation(ctx context.Context, input providerops.CreateInput, currentID, queuedID string, now time.Time) (providerops.Operation, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil || input.Kind != purchaseops.OperationEarlyActivation || input.ActorUserID != input.OwnerUserID || len(input.Items) != 1 || input.Items[0].TargetType != "purchase" || input.Items[0].TargetID != queuedID {
		return providerops.Operation{}, ErrConflict
	}
	now = now.UTC()
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if operation, found, err := memberOperationReplayTx(ctx, tx, input, now); found || err != nil {
		if err == nil {
			err = tx.Commit()
		}
		return operation, err
	}
	current, err := activeControlPurchaseTx(ctx, tx, input.OwnerUserID, currentID, now)
	if err != nil {
		return providerops.Operation{}, err
	}
	queued, err := queuedControlPurchaseTx(ctx, tx, input.OwnerUserID, queuedID)
	if err != nil {
		return providerops.Operation{}, err
	}
	if conflict, err := comboControlConflictFrom(ctx, tx, input.OwnerUserID); err != nil || conflict {
		if err == nil {
			err = ErrConflict
		}
		return providerops.Operation{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',valid_until=?,auto_renew_enabled=0,updated_at=? WHERE id=? AND status='active'`, stamp(now), stamp(now), currentID); err != nil {
		return providerops.Operation{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchase_rollovers SET status='zero',credited_txb_minor=0,exception_code='COMBO_FORFEITED',updated_at=?
 WHERE purchase_id=? AND status IN ('pending','processing','calculated')`, stamp(now), currentID); err != nil {
		return providerops.Operation{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind='rollover_finalize' AND json_extract(payload,'$.purchaseId')=? AND status<>'processing'`, currentID); err != nil {
		return providerops.Operation{}, err
	}
	if err := shiftRefundQueueTx(ctx, tx, input.OwnerUserID, queued.ValidFrom, now.Sub(queued.ValidFrom), now); err != nil {
		return providerops.Operation{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',traffic_reset_phase='pending',updated_at=? WHERE id=? AND status='queued'`, stamp(now), queuedID); err != nil {
		return providerops.Operation{}, err
	}
	if err := retirePurchaseContinuityTx(ctx, tx, queuedID, now); err != nil {
		return providerops.Operation{}, err
	}
	if err := applyPendingExtensionsToActivationTx(ctx, tx, queuedID, now); err != nil {
		return providerops.Operation{}, err
	}
	operation, _, err := createProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, err
	}
	if err := auditComboControlTx(ctx, tx, input.OwnerUserID, "member.combo_forfeited", currentID, map[string]any{
		"successorId": queuedID, "previousExpiry": current.ValidUntil, "newStart": now, "operationId": operation.Receipt.ID, "txbMovementMinor": 0}, now); err != nil {
		return providerops.Operation{}, err
	}
	return operation, tx.Commit()
}
