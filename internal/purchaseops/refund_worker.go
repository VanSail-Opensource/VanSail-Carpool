package purchaseops

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *Worker) handleRefund(ctx context.Context, operation providerops.Operation) error {
	operation, item, err := w.begin(ctx, operation)
	if err != nil {
		return err
	}
	if item.Status != providerops.StatusQueued && item.Status != providerops.StatusProcessing {
		return w.finishItem(ctx, operation, item)
	}
	facts, user, err := w.loadTarget(ctx, operation, item)
	if err != nil {
		return err
	}
	if user.RemnaUserID == nil {
		return w.complete(ctx, operation, item, providerops.StatusFailed, ReasonRemoteUnlinked)
	}
	remoteID := *user.RemnaUserID
	if item.Status == providerops.StatusQueued {
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
		if err != nil {
			return err
		}
	} else {
		state, stateErr := refundState(ctx, w.remote, remoteID, facts.Purchase.ID)
		if stateErr != nil {
			return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_RECONCILIATION_FAILED")
		}
		if state.Quiesced {
			return w.finishQuiescedRefund(ctx, operation, item, facts, remoteID, state.UsedTrafficBytes)
		}
	}
	quiesceErr := w.remote.QuiesceMemberOperation(ctx, remoteID)
	if quiesceErr != nil {
		state, stateErr := refundState(ctx, w.remote, remoteID, facts.Purchase.ID)
		if stateErr != nil {
			return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_QUIESCE_AMBIGUOUS")
		}
		if !state.Quiesced {
			if w.remote.DefinitiveMutationFailure(quiesceErr) {
				return w.complete(ctx, operation, item, providerops.StatusFailed, "REFUND_QUIESCE_FAILED")
			}
			return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_QUIESCE_AMBIGUOUS")
		}
	}
	state, err := refundState(ctx, w.remote, remoteID, facts.Purchase.ID)
	if err != nil {
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_USAGE_RECHECK_FAILED")
	}
	return w.finishQuiescedRefund(ctx, operation, item, facts, remoteID, state.UsedTrafficBytes)
}

func (w *Worker) finishQuiescedRefund(ctx context.Context, operation providerops.Operation, item providerops.Item, facts PurchaseFacts, remoteID string, used int64) error {
	if used != 0 {
		if err := w.remote.RestoreMemberOperation(ctx, remoteID, facts.Purchase); err != nil {
			return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_RESTORE_FAILED")
		}
		return w.complete(ctx, operation, item, providerops.StatusFailed, ReasonTrafficUsed)
	}
	_, err := w.repository.FinalizeMemberRefund(ctx, operation.Receipt.ID, facts.Purchase.ID, w.now().UTC())
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrIneligible) {
		return err
	}
	if restoreErr := w.remote.RestoreMemberOperation(ctx, remoteID, facts.Purchase); restoreErr != nil {
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_RESTORE_FAILED")
	}
	return w.complete(ctx, operation, item, providerops.StatusFailed, "REFUND_STATE_CONFLICT")
}
