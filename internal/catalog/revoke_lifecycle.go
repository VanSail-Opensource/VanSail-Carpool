package catalog

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func revokeItem(ctx context.Context, repository revokeOperationRepository, operationID string) (providerops.Item, error) {
	items, err := repository.ProviderOperationItems(ctx, operationID)
	if err != nil || len(items) != 1 || items[0].TargetType != "user" {
		if err == nil {
			err = errors.New("subscription revoke operation has an invalid target")
		}
		return providerops.Item{}, err
	}
	return items[0], nil
}

func (s *Service) beginRevoke(ctx context.Context, repository revokeOperationRepository, operation providerops.Operation,
	item providerops.Item) (providerops.Operation, providerops.Item, error) {
	var err error
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, err
		}
	}
	if item.Status == providerops.StatusQueued {
		item, err = repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, s.now().UTC())
	}
	return operation, item, err
}

func (s *Service) finishRevoke(ctx context.Context, repository revokeOperationRepository, operation providerops.Operation,
	item providerops.Item, status providerops.Status, code string) error {
	if operation.Receipt.Status == string(providerops.StatusQueued) || item.Status == providerops.StatusQueued {
		var err error
		operation, item, err = s.beginRevoke(ctx, repository, operation, item)
		if err != nil {
			return err
		}
	}
	completion := providerops.Completion{Status: status, ErrorCode: code}
	if item.Status == providerops.StatusProcessing {
		if _, err := repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, s.now().UTC()); err != nil {
			return err
		}
	}
	_, err := repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, s.now().UTC())
	return err
}

func (s *Service) invalidateDashboard(userID string) {
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()
}
