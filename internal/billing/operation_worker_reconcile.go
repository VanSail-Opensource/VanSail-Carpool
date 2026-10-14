package billing

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *OperationWorker) complete(ctx context.Context, operation providerops.Operation, item providerops.Item,
	status providerops.Status, code, providerReference string, providerMessage ...string) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	now := w.now().UTC()
	resultFields := map[string]string{"paymentOrderId": item.TargetID}
	if len(providerMessage) > 0 && providerMessage[0] != "" {
		resultFields["providerMessage"] = providerMessage[0]
	}
	result, err := json.Marshal(resultFields)
	if err != nil {
		return err
	}
	completion := providerops.Completion{Status: status, ErrorCode: code,
		ProviderReference: providerReference, ResultJSON: string(result)}
	if item.Status == providerops.StatusQueued {
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, now)
		if err != nil {
			return w.concurrentCompletion(ctx, operation.Receipt.ID, err)
		}
	}
	if item.Status == providerops.StatusProcessing {
		if _, err := w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, now); err != nil {
			return w.concurrentCompletion(ctx, operation.Receipt.ID, err)
		}
	}
	_, err = w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, now)
	if err != nil {
		return w.concurrentCompletion(ctx, operation.Receipt.ID, err)
	}
	return nil
}

func (w *OperationWorker) concurrentCompletion(ctx context.Context, operationID string, original error) error {
	current, err := w.repository.ProviderOperationByID(ctx, operationID)
	if err == nil && providerops.Terminal(providerops.Status(current.Receipt.Status)) {
		return nil
	}
	return original
}

func (w *OperationWorker) reconcile(ctx context.Context, operation providerops.Operation, item providerops.Item) error {
	order, err := w.repository.PaymentOrderByID(ctx, item.TargetID)
	if err != nil {
		return w.complete(ctx, operation, item, providerops.StatusPendingReview,
			"PAYMENT_LOCAL_STATE_UNAVAILABLE", "")
	}
	return w.reconcileOrder(ctx, operation, item, order)
}

func (w *OperationWorker) reconcileOrder(ctx context.Context, operation providerops.Operation,
	item providerops.Item, order model.PaymentOrder) error {
	reference := paymentOrderReference(order)
	switch operation.Receipt.Kind {
	case OperationCreateKind:
		return w.reconcileCreate(ctx, operation, item, order, reference)
	case OperationCancelKind:
		return w.reconcileCancellation(ctx, operation, item, order, reference)
	default:
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_OPERATION_UNSUPPORTED", reference)
	}
}

func (w *OperationWorker) reconcileCreate(ctx context.Context, operation providerops.Operation,
	item providerops.Item, order model.PaymentOrder, reference string) error {
	// Local quote reservation is transactional and idempotent; an interrupted
	// create can safely resume without repeating an external mutation.
	if order.Provider == "builtin_usdt" && order.Status == "creating" && order.CancelledAt == nil {
		return w.create(ctx, operation, item)
	}
	if order.PaidAt != nil || order.Status == "paid" || order.Status == "refunded" || order.Status == "pending" {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", reference)
	}
	if order.Status == "failed" {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_CREATE_REJECTED", reference)
	}
	if reference != "" || order.PaymentURL != nil || order.QRPayload != nil || order.ReceivingAddress != nil {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", reference)
	}
	return w.complete(ctx, operation, item, providerops.StatusPendingReview, "PAYMENT_CREATE_OUTCOME_UNKNOWN", reference)
}

func (w *OperationWorker) reconcileCancellation(ctx context.Context, operation providerops.Operation,
	item providerops.Item, order model.PaymentOrder, reference string) error {
	switch order.ProviderCancelStatus {
	case "cancelled", "unsupported":
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", reference)
	case "pending_review", "failed":
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "PAYMENT_CANCEL_AMBIGUOUS", reference)
	}
	if order.PaidAt != nil || order.Status == "paid" || order.Status == "refunded" {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_ALREADY_SETTLED", reference)
	}
	if order.Provider != "bepusdt" || order.ProviderTradeID == nil {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", reference)
	}
	return w.complete(ctx, operation, item, providerops.StatusPendingReview, "PAYMENT_CANCEL_OUTCOME_UNKNOWN", reference)
}

func paymentOrderReference(order model.PaymentOrder) string {
	if order.ProviderTradeID != nil {
		return *order.ProviderTradeID
	}
	if order.ProviderChargeID != nil {
		return *order.ProviderChargeID
	}
	return ""
}
