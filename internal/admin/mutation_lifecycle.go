package admin

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *MutationWorker) start(ctx context.Context, operation providerops.Operation,
	targetType string) (providerops.Operation, providerops.Item, bool, error) {
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != targetType {
		if err == nil {
			err = errors.New("administrator mutation operation has an invalid target")
		}
		return providerops.Operation{}, providerops.Item{}, false, err
	}
	item := items[0]
	fresh := item.Status == providerops.StatusQueued
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, false, err
		}
	}
	if fresh {
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	}
	return operation, item, fresh, err
}

func (w *MutationWorker) complete(ctx context.Context, operation providerops.Operation, item providerops.Item,
	status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code}
	if item.Status == providerops.StatusProcessing {
		if _, err := w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, w.now().UTC()); err != nil {
			return err
		}
	}
	_, err := w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, w.now().UTC())
	return err
}
