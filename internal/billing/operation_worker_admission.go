package billing

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *OperationWorker) loadTarget(ctx context.Context, operation providerops.Operation, item providerops.Item) (model.PaymentOrder, error) {
	order, err := w.repository.PaymentOrderByID(ctx, item.TargetID)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if order.ID != item.TargetID || order.UserID != operation.OwnerUserID {
		return model.PaymentOrder{}, database.ErrConflict
	}
	return order, nil
}

// Exhausted local reads must leave a visible, durable review outcome instead of
// a permanently processing receipt. A provider attempt is never repeated here.
func (w *OperationWorker) finishExhausted(ctx context.Context, operationID string, cause error) error {
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	operation, err := w.repository.ProviderOperationByID(finish, operationID)
	if err != nil {
		return errors.Join(cause, err)
	}
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = w.repository.BeginProviderOperationAttempt(finish, operationID, w.now().UTC())
		if err != nil {
			return w.concurrentCompletion(finish, operationID, errors.Join(cause, err))
		}
	}
	items, err := w.repository.ProviderOperationItems(finish, operationID)
	if err != nil {
		return errors.Join(cause, err)
	}
	if len(items) != 1 || items[0].TargetType != "payment_order" || items[0].TargetID == "" {
		return cause
	}
	item := items[0]
	if providerops.Terminal(item.Status) {
		return w.completeRecorded(finish, operation, item)
	}
	return w.complete(finish, operation, item, providerops.StatusPendingReview, "PAYMENT_LOCAL_STATE_UNAVAILABLE", "")
}
