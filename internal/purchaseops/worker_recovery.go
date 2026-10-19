package purchaseops

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// An exhausted read/admission retry must not leave its receipt processing
// forever. Only a never-attempted reset can be safely compensated here.
func (w *Worker) finishExhausted(ctx context.Context, operation providerops.Operation, cause error) error {
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if loader, ok := w.repository.(providerops.OperationLoader); ok {
		current, err := loader.ProviderOperationByID(finish, operation.Receipt.ID)
		if err != nil {
			return errors.Join(cause, err)
		}
		operation = current
	}
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	items, err := w.repository.ProviderOperationItems(finish, operation.Receipt.ID)
	if err != nil {
		return errors.Join(cause, err)
	}
	if len(items) != 1 || items[0].TargetType != "purchase" {
		_, err := w.repository.CompleteProviderOperation(finish, operation.Receipt.ID, providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "PURCHASE_OPERATION_TARGET_INVALID", ResultJSON: "{}"}, w.now().UTC())
		return err
	}
	item := items[0]
	if item.Status == providerops.StatusQueued {
		if operation.Receipt.Kind == OperationResetKind {
			if _, err := w.repository.BeginProviderOperationItemAttempt(finish, operation.Receipt.ID, item.Key, w.now().UTC()); err != nil {
				return errors.Join(cause, err)
			}
			return w.repository.CompensateTrafficReset(finish, operation.Receipt.ID, "RESET_ADMISSION_UNAVAILABLE", w.now().UTC())
		}
		return w.complete(finish, operation, item, providerops.StatusFailed, "REFUND_ADMISSION_UNAVAILABLE")
	}
	if item.Status == providerops.StatusProcessing {
		code := "RESET_RECONCILIATION_FAILED"
		if operation.Receipt.Kind == OperationRefundKind {
			code = "REFUND_RECONCILIATION_FAILED"
		}
		return w.complete(finish, operation, item, providerops.StatusPendingReview, code)
	}
	return w.finishItem(finish, operation, item)
}
