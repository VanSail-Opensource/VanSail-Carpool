package statistics

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// HostWorkerRepository owns durable attempt and completion transitions.
type HostWorkerRepository interface {
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	ListProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
}

// HostRemarkWorker applies a scheduled host PATCH at most once.
type HostRemarkWorker struct {
	repository HostWorkerRepository
	provider   Provider
	now        func() time.Time
}

// NewHostRemarkWorker creates the durable scheduled mutation handler.
func NewHostRemarkWorker(repository HostWorkerRepository, provider Provider) *HostRemarkWorker {
	return &HostRemarkWorker{repository: repository, provider: provider, now: time.Now}
}

// HandleProviderOperation reconciles interrupted attempts before deciding state.
func (w *HostRemarkWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	target, item, err := w.target(ctx, operation, job)
	if err != nil {
		return err
	}
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return err
		}
	}
	freshAttempt := item.Status == providerops.StatusQueued
	if freshAttempt {
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
		if err != nil {
			return err
		}
	}
	if providerops.Terminal(item.Status) {
		return w.completeOperation(ctx, operation.Receipt.ID, item.Status, item.ErrorCode)
	}
	current, err := w.currentRemark(ctx, target.HostUUID)
	if err != nil {
		return err
	}
	if current == target.Remark {
		return w.complete(ctx, operation.Receipt.ID, item.Key, providerops.StatusSucceeded, "")
	}
	if !freshAttempt {
		return w.complete(ctx, operation.Receipt.ID, item.Key, providerops.StatusPendingReview, "HOST_UPDATE_OUTCOME_AMBIGUOUS")
	}
	if err := w.provider.UpdateHostRemark(ctx, target.HostUUID, target.Remark); err == nil {
		return w.complete(ctx, operation.Receipt.ID, item.Key, providerops.StatusSucceeded, "")
	}
	current, readErr := w.currentRemark(ctx, target.HostUUID)
	if readErr == nil && current == target.Remark {
		return w.complete(ctx, operation.Receipt.ID, item.Key, providerops.StatusSucceeded, "")
	}
	return w.complete(ctx, operation.Receipt.ID, item.Key, providerops.StatusPendingReview, "HOST_UPDATE_OUTCOME_AMBIGUOUS")
}

func (w *HostRemarkWorker) target(ctx context.Context, operation providerops.Operation, job model.OutboxJob) (hostRemarkTarget, providerops.Item, error) {
	items, err := w.repository.ListProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "remnawave_host" {
		return hostRemarkTarget{}, providerops.Item{}, errors.New("host remark operation has an invalid target")
	}
	sealed, err := jobpayload.TargetID(job, "sealedTarget")
	if err != nil {
		return hostRemarkTarget{}, providerops.Item{}, err
	}
	target, err := decodeHostRemarkTarget(sealed)
	if err != nil || target.HostUUID != items[0].TargetID {
		return hostRemarkTarget{}, providerops.Item{}, errors.New("host remark operation target could not be verified")
	}
	return target, items[0], nil
}

func (w *HostRemarkWorker) currentRemark(ctx context.Context, hostUUID string) (string, error) {
	hosts, err := w.provider.Hosts(ctx)
	if err != nil {
		return "", err
	}
	for _, host := range hosts {
		if host.UUID == hostUUID {
			return host.Remark, nil
		}
	}
	return "", errors.New("Remnawave host is unavailable")
}

func (w *HostRemarkWorker) complete(ctx context.Context, operationID, itemKey string, status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code}
	if _, err := w.repository.CompleteProviderOperationItem(ctx, operationID, itemKey, completion, w.now().UTC()); err != nil {
		return err
	}
	return w.completeOperation(ctx, operationID, status, code)
}

func (w *HostRemarkWorker) completeOperation(ctx context.Context, operationID string, status providerops.Status, code string) error {
	_, err := w.repository.CompleteProviderOperation(ctx, operationID, providerops.Completion{
		Status: status, ErrorCode: code,
	}, w.now().UTC())
	return err
}

var _ providerops.Handler = (*HostRemarkWorker)(nil)
