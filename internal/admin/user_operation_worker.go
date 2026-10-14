package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// UserOperationRepository is the durable attempt and exact-state surface.
type UserOperationRepository interface {
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	ListProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	UserByID(context.Context, string) (model.User, error)
	DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error)
}

type notificationItemCompleter interface {
	CompleteProviderOperationItemAndReleaseNotifications(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
}

// EntitlementRemote replaces the full upstream entitlement through its queue.
type EntitlementRemote interface {
	ApplyEntitlement(context.Context, string, int64, string, []string, time.Time) error
	RemoveEntitlement(context.Context, string) error
}

// UserOperationWorker reconciles audited administrator commands upstream.
type UserOperationWorker struct {
	repository UserOperationRepository
	remote     EntitlementRemote
	now        func() time.Time
}

// NewUserOperationWorker constructs the shared-dispatch admin handler.
func NewUserOperationWorker(repository UserOperationRepository, remote EntitlementRemote) *UserOperationWorker {
	return &UserOperationWorker{repository: repository, remote: remote, now: time.Now}
}

// HandleProviderOperation applies each target at most once per recorded attempt.
func (w *UserOperationWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		started, err := w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return err
		}
		operation = started
	}
	if operation.Receipt.Status != string(providerops.StatusProcessing) {
		return nil
	}
	items, err := w.repository.ListProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	completed := make([]providerops.Item, 0, len(items))
	for _, item := range items {
		result, err := w.processItem(ctx, operation, item)
		if err != nil {
			return err
		}
		completed = append(completed, result)
	}
	completion := summarizeAdminItems(completed)
	_, err = w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, w.now().UTC())
	return err
}

func (w *UserOperationWorker) processItem(ctx context.Context, operation providerops.Operation, item providerops.Item) (providerops.Item, error) {
	if providerops.Terminal(item.Status) {
		return item, nil
	}
	if item.Status == providerops.StatusProcessing {
		return w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, providerops.Completion{
			Status: providerops.StatusPendingReview, ErrorCode: "INTERRUPTED_PROVIDER_ATTEMPT",
			ResultJSON: `{"reason":"provider outcome was not durably recorded"}`,
		}, w.now().UTC())
	}
	started, err := w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	if err != nil {
		return providerops.Item{}, err
	}
	completion := w.applyExactState(ctx, started.TargetID)
	if completion.Status == providerops.StatusSucceeded {
		if repository, ok := w.repository.(notificationItemCompleter); ok {
			return repository.CompleteProviderOperationItemAndReleaseNotifications(ctx, operation.Receipt.ID,
				item.Key, completion, w.now().UTC())
		}
	}
	return w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, w.now().UTC())
}

func (w *UserOperationWorker) applyExactState(ctx context.Context, userID string) providerops.Completion {
	now := w.now().UTC()
	user, err := w.repository.UserByID(ctx, userID)
	if err != nil {
		return providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "LOCAL_STATE_UNAVAILABLE"}
	}
	desired, err := w.repository.DesiredEntitlement(ctx, userID, now)
	if err != nil {
		return providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "LOCAL_STATE_UNAVAILABLE"}
	}
	if user.RemnaUserID == nil {
		return providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: `{"providerMutation":"not_required"}`}
	}
	if desired == nil {
		err = w.remote.RemoveEntitlement(ctx, *user.RemnaUserID)
	} else {
		err = w.remote.ApplyEntitlement(ctx, *user.RemnaUserID, desired.TrafficLimitBytes,
			desired.ResetStrategy, desired.SquadUUIDs, desired.ValidUntil)
	}
	if err != nil {
		return providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "UPSTREAM_OUTCOME_AMBIGUOUS",
			ResultJSON: `{"reason":"exact provider state must be reconciled before resolution"}`}
	}
	return providerops.Completion{Status: providerops.StatusSucceeded}
}

func summarizeAdminItems(items []providerops.Item) providerops.Completion {
	counts := map[providerops.Status]int{}
	for _, item := range items {
		counts[item.Status]++
	}
	status := providerops.StatusSucceeded
	if len(items) == 0 || counts[providerops.StatusFailed] == len(items) {
		status = providerops.StatusFailed
	} else if counts[providerops.StatusPendingReview] == len(items) {
		status = providerops.StatusPendingReview
	} else if counts[providerops.StatusSucceeded] != len(items) {
		status = providerops.StatusPartial
	}
	result, err := json.Marshal(map[string]int{"targets": len(items), "succeeded": counts[providerops.StatusSucceeded],
		"failed": counts[providerops.StatusFailed], "pendingReview": counts[providerops.StatusPendingReview]})
	if err != nil {
		return providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "RESULT_ENCODING_FAILED"}
	}
	return providerops.Completion{Status: status, ResultJSON: string(result)}
}

var _ providerops.Handler = (*UserOperationWorker)(nil)
