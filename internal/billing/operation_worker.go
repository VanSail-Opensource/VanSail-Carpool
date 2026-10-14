package billing

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type paymentWorkerRepository interface {
	PaymentOrderByID(context.Context, string) (model.PaymentOrder, error)
	ProviderOperationByID(context.Context, string) (providerops.Operation, error)
	UserByID(context.Context, string) (model.User, error)
	FailPaymentOrder(context.Context, string) error
	SetPaymentProviderCancellation(context.Context, string, string, time.Time) error
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
}

// OperationWorker performs payment provider writes after durable intent is recorded.
type OperationWorker struct {
	service    *Service
	repository paymentWorkerRepository
	now        func() time.Time
}

// NewOperationWorker creates the durable payment create/cancel handler.
func NewOperationWorker(service *Service) (*OperationWorker, error) {
	repository, ok := service.repository.(paymentWorkerRepository)
	if !ok {
		return nil, errors.New("payment repository does not support durable operations")
	}
	return &OperationWorker{service: service, repository: repository, now: time.Now}, nil
}

// HandleProviderOperation executes once, or reconciles an interrupted attempt without retrying it.
func (w *OperationWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return err
		}
	}
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].TargetType != "payment_order" {
		_, err := w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{
			Status: providerops.StatusFailed, ErrorCode: "PAYMENT_OPERATION_TARGET_INVALID", ResultJSON: "{}",
		}, w.now().UTC())
		return err
	}
	item := items[0]
	if providerops.Terminal(item.Status) {
		return w.complete(ctx, operation, item, item.Status, item.ErrorCode, item.ProviderReference)
	}
	if item.Status == providerops.StatusProcessing {
		return w.reconcile(ctx, operation, item)
	}
	item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	if err != nil {
		return err
	}
	switch operation.Receipt.Kind {
	case OperationCreateKind:
		return w.create(ctx, operation, item)
	case OperationCancelKind:
		return w.cancel(ctx, operation, item)
	default:
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_OPERATION_UNSUPPORTED", "")
	}
}

func (w *OperationWorker) create(ctx context.Context, operation providerops.Operation, item providerops.Item) error {
	order, err := w.repository.PaymentOrderByID(ctx, item.TargetID)
	if err != nil {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_ORDER_NOT_FOUND", "")
	}
	if order.Status != "creating" || order.CancelledAt != nil {
		return w.reconcileOrder(ctx, operation, item, order)
	}
	user, err := w.repository.UserByID(ctx, order.UserID)
	if err != nil {
		if failErr := w.repository.FailPaymentOrder(ctx, order.ID); failErr != nil {
			return failErr
		}
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_OWNER_NOT_FOUND", "")
	}
	request, err := w.service.providerCreateRequest(ctx, user, order)
	if err != nil {
		if failErr := w.repository.FailPaymentOrder(ctx, order.ID); failErr != nil {
			return failErr
		}
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_CONFIGURATION_INVALID", "")
	}
	checkout, err := w.service.gateway.Create(ctx, request)
	if err != nil {
		if code, rejected := builtinCreateRejection(order.Provider, err); rejected {
			if err := w.repository.FailPaymentOrder(ctx, order.ID); err != nil {
				return err
			}
			return w.complete(ctx, operation, item, providerops.StatusFailed, code, "")
		}
		if message, rejected := providerCreateRejection(err); rejected {
			if failErr := w.repository.FailPaymentOrder(ctx, order.ID); failErr != nil {
				return failErr
			}
			return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_CREATE_REJECTED", "", message)
		}
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "PAYMENT_CREATE_AMBIGUOUS", "")
	}
	if _, err := w.service.storeCheckout(ctx, order, checkout); err != nil {
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "PAYMENT_CHECKOUT_REJECTED", reference(checkout))
	}
	return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", reference(checkout))
}

func (w *OperationWorker) cancel(ctx context.Context, operation providerops.Operation, item providerops.Item) error {
	order, err := w.repository.PaymentOrderByID(ctx, item.TargetID)
	if err != nil {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_ORDER_NOT_FOUND", "")
	}
	canceller, supported := w.service.gateway.(CancellationGateway)
	if !supported || order.Provider != "bepusdt" || order.ProviderTradeID == nil {
		if err := w.repository.SetPaymentProviderCancellation(ctx, order.ID, "unsupported", w.now().UTC()); err != nil {
			return err
		}
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", "")
	}
	if err := canceller.Cancel(ctx, order); err != nil {
		if persistErr := w.repository.SetPaymentProviderCancellation(ctx, order.ID, "pending_review", w.now().UTC()); persistErr != nil {
			return persistErr
		}
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "PAYMENT_CANCEL_AMBIGUOUS", *order.ProviderTradeID)
	}
	if err := w.repository.SetPaymentProviderCancellation(ctx, order.ID, "cancelled", w.now().UTC()); err != nil {
		return err
	}
	return w.complete(ctx, operation, item, providerops.StatusSucceeded, "", *order.ProviderTradeID)
}

func reference(checkout ProviderCheckout) string {
	if checkout.TradeID == nil {
		return ""
	}
	return *checkout.TradeID
}
