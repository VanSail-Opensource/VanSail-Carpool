package purchaseops

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *Worker) handleReset(ctx context.Context, operation providerops.Operation) error {
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
		if item.Status == providerops.StatusQueued {
			item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
		}
		if err != nil {
			return err
		}
		return w.repository.CompensateTrafficReset(ctx, operation.Receipt.ID, ReasonRemoteUnlinked, w.now().UTC())
	}
	if item.Status == providerops.StatusProcessing {
		return w.reconcileReset(ctx, operation, item, *user.RemnaUserID)
	}
	quote, quoteErr := QuoteTrafficReset(facts, operation.OwnerUserID, w.now().UTC())
	if quoteErr != nil || !quote.Eligible {
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
		if err != nil {
			return err
		}
		return w.repository.CompensateTrafficReset(ctx, operation.Receipt.ID, ReasonNotActive, w.now().UTC())
	}
	before, err := w.remote.MemberOperationState(ctx, *user.RemnaUserID)
	if err != nil {
		item, beginErr := w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
		if beginErr != nil {
			return beginErr
		}
		_ = item
		return w.repository.CompensateTrafficReset(ctx, operation.Receipt.ID, "RESET_PRECHECK_FAILED", w.now().UTC())
	}
	item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	if err != nil {
		return err
	}
	callErr := w.remote.ResetTraffic(ctx, *user.RemnaUserID)
	if callErr == nil {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	after, reconcileErr := w.remote.MemberOperationState(ctx, *user.RemnaUserID)
	if reconcileErr == nil && resetChanged(before.LastTrafficResetAt, after.LastTrafficResetAt) {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	if w.remote.DefinitiveMutationFailure(callErr) {
		return w.repository.CompensateTrafficReset(ctx, operation.Receipt.ID, "RESET_REJECTED", w.now().UTC())
	}
	return w.complete(ctx, operation, item, providerops.StatusPendingReview, "RESET_OUTCOME_AMBIGUOUS")
}

func (w *Worker) reconcileReset(ctx context.Context, operation providerops.Operation, item providerops.Item, remoteID string) error {
	state, err := w.remote.MemberOperationState(ctx, remoteID)
	if err != nil {
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "RESET_RECONCILIATION_FAILED")
	}
	if item.AttemptStartedAt != nil && resetAtOrAfter(state.LastTrafficResetAt, *item.AttemptStartedAt) {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	return w.complete(ctx, operation, item, providerops.StatusPendingReview, "RESET_OUTCOME_AMBIGUOUS")
}

func resetChanged(before, after *time.Time) bool {
	return after != nil && (before == nil || after.After(*before))
}

func resetAtOrAfter(resetAt *time.Time, started time.Time) bool {
	return resetAt != nil && !resetAt.Before(started.Add(-time.Second))
}
