package entitlements

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type memberPurchaseActivator interface {
	ApplyMemberPurchase(context.Context, model.User, model.Purchase) error
}

type purchaseSyncRevisionRepository interface {
	MarkPurchaseSyncResultIfUnchanged(context.Context, model.Purchase, int64, bool, time.Time) error
}

type nativePurchaseAttemptError struct {
	purchase   model.Purchase
	generation int64
	cause      error
}

func (e *nativePurchaseAttemptError) Error() string {
	return "apply native purchase: " + e.cause.Error()
}
func (e *nativePurchaseAttemptError) Unwrap() error { return e.cause }

// Native period replacement is already idempotent for an exact purchase. It
// must never run the historical account-wide quiesce/reset protocol, which can
// target a successor and erase its current-period counters on a delayed replay.
func (w *Worker) applyNativePurchase(ctx context.Context, native memberPurchaseActivator, user model.User, purchaseID string) error {
	repository, ok := w.repository.(purchaseSyncRevisionRepository)
	if !ok {
		return errors.New("purchase activation revision repository is unavailable")
	}
	purchase, err := w.repository.PurchaseByID(ctx, purchaseID)
	if err != nil {
		return err
	}
	if purchase.ID != purchaseID || purchase.UserID != user.ID {
		return errors.New("purchase activation owner changed")
	}
	if purchase.Status == "cancelled" || purchase.Status == "expired" {
		return nil
	}
	now := w.now().UTC()
	if !now.Before(purchase.ValidUntil) {
		return w.repository.ExpirePurchase(ctx, purchase.ID, now)
	}
	if err := native.ApplyMemberPurchase(ctx, user, purchase); err != nil {
		return &nativePurchaseAttemptError{purchase: purchase, generation: user.AccountGeneration, cause: err}
	}
	return repository.MarkPurchaseSyncResultIfUnchanged(ctx, purchase, user.AccountGeneration, true, w.now().UTC())
}

func (w *Worker) markNativeActivationFailure(ctx context.Context, failure error) error {
	var attempt *nativePurchaseAttemptError
	if !errors.As(failure, &attempt) {
		return failure
	}
	repository, ok := w.repository.(purchaseSyncRevisionRepository)
	if !ok {
		return failure
	}
	if err := repository.MarkPurchaseSyncResultIfUnchanged(ctx, attempt.purchase, attempt.generation, false, w.now().UTC()); err != nil {
		return errors.Join(failure, fmt.Errorf("mark current native activation failure: %w", err))
	}
	return failure
}
