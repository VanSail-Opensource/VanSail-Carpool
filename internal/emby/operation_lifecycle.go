package emby

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *OperationService) start(ctx context.Context, operation providerops.Operation,
	targetType string) (providerops.Operation, providerops.Item, bool, error) {
	items, err := s.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != targetType {
		if err == nil {
			err = errors.New("Emby operation has an invalid target")
		}
		return providerops.Operation{}, providerops.Item{}, false, err
	}
	item := items[0]
	fresh := item.Status == providerops.StatusQueued
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = s.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, false, err
		}
	}
	if fresh {
		item, err = s.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, s.now().UTC())
	}
	return operation, item, fresh, err
}

func (s *OperationService) complete(ctx context.Context, operation providerops.Operation, item providerops.Item,
	status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code}
	if item.Status == providerops.StatusProcessing {
		if _, err := s.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, s.now().UTC()); err != nil {
			return err
		}
	}
	_, err := s.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, s.now().UTC())
	return err
}
