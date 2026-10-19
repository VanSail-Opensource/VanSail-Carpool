package purchaseops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// WorkerRepository is the durable phase and atomic settlement surface.
type WorkerRepository interface {
	MemberPurchaseFacts(context.Context, string) (PurchaseFacts, error)
	UserByID(context.Context, string) (model.User, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	CompensateTrafficReset(context.Context, string, string, time.Time) error
	FinalizeMemberRefund(context.Context, string, string, time.Time) (RefundResult, error)
}

type notificationCompletionRepository interface {
	CompleteProviderOperationItemAndReleaseNotifications(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
}

// Worker reconciles and settles reset/refund provider operations.
type Worker struct {
	repository WorkerRepository
	remote     Remote
	now        func() time.Time
}

// NewWorker creates a member operation handler.
func NewWorker(repository WorkerRepository, remote Remote) *Worker {
	return &Worker{repository: repository, remote: remote, now: time.Now}
}

// HandleProviderOperation dispatches supported member operation kinds.
func (w *Worker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	var err error
	switch operation.Receipt.Kind {
	case OperationResetKind:
		err = w.handleReset(ctx, operation)
	case OperationRefundKind:
		err = w.handleRefund(ctx, operation)
	default:
		return fmt.Errorf("unsupported member operation kind %q", operation.Receipt.Kind)
	}
	if err != nil && job.Attempts >= 10 {
		return w.finishExhausted(ctx, operation, err)
	}
	return err
}

func (w *Worker) begin(ctx context.Context, operation providerops.Operation) (providerops.Operation, providerops.Item, error) {
	now := w.now().UTC()
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now)
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, err
		}
	}
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return providerops.Operation{}, providerops.Item{}, err
	}
	if len(items) != 1 || items[0].TargetType != "purchase" || items[0].TargetID == "" {
		return providerops.Operation{}, providerops.Item{}, errors.New("member operation has an invalid purchase target")
	}
	return operation, items[0], nil
}

func (w *Worker) complete(ctx context.Context, operation providerops.Operation, item providerops.Item, status providerops.Status, code string) error {
	now := w.now().UTC()
	completion := providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}
	if item.Status == providerops.StatusQueued {
		var err error
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, now)
		if err != nil {
			return err
		}
	}
	if item.Status == providerops.StatusProcessing {
		var err error
		if repository, ok := w.repository.(notificationCompletionRepository); ok && status == providerops.StatusSucceeded {
			_, err = repository.CompleteProviderOperationItemAndReleaseNotifications(ctx, operation.Receipt.ID, item.Key, completion, now)
		} else {
			_, err = w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, now)
		}
		if err != nil {
			return err
		}
	}
	_, err := w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, now)
	return err
}

func (w *Worker) finishItem(ctx context.Context, operation providerops.Operation, item providerops.Item) error {
	switch item.Status {
	case providerops.StatusSucceeded, providerops.StatusFailed, providerops.StatusCompensated, providerops.StatusPendingReview:
		return w.complete(ctx, operation, item, item.Status, item.ErrorCode)
	default:
		return nil
	}
}

func (w *Worker) loadTarget(ctx context.Context, operation providerops.Operation, item providerops.Item) (PurchaseFacts, model.User, error) {
	facts, err := w.repository.MemberPurchaseFacts(ctx, item.TargetID)
	if err != nil {
		return PurchaseFacts{}, model.User{}, err
	}
	if facts.Purchase.UserID != operation.OwnerUserID {
		return PurchaseFacts{}, model.User{}, ErrNotFound
	}
	user, err := w.repository.UserByID(ctx, operation.OwnerUserID)
	return facts, user, err
}
