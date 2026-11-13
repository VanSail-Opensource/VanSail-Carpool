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
	if providerops.Terminal(item.Status) {
		return operation, item, false, s.completeRecordedItem(ctx, operation, item)
	}
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
	if providerops.Terminal(item.Status) {
		return s.completeRecordedItem(ctx, operation, item)
	}
	completion := providerops.Completion{Status: status, ErrorCode: code}
	if item.Status == providerops.StatusProcessing {
		if _, err := s.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, s.now().UTC()); err != nil {
			if completed, retainedErr := s.completeRetainedItem(ctx, operation); completed {
				return retainedErr
			}
			return err
		}
	}
	_, err := s.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, s.now().UTC())
	return err
}

func (s *OperationService) completeRetainedItem(ctx context.Context, operation providerops.Operation) (bool, error) {
	targetType := "emby_user"
	switch operation.Receipt.Kind {
	case providerops.KindEmbySetup, providerops.KindEmbyPreferences, providerops.KindEmbyPassword:
	case providerops.KindEmbyProvisionRetry:
		targetType = "emby_account"
	default:
		return false, nil
	}
	items, err := s.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return false, err
	}
	if len(items) != 1 || items[0].TargetType != targetType || !providerops.Terminal(items[0].Status) {
		return false, nil
	}
	return true, s.completeRecordedItem(ctx, operation, items[0])
}

// Once an item has a retained outcome, repairing its parent receipt requires no
// password decryption, mutable account lookup, or repeated remote operation.
func (s *OperationService) completeRecordedItem(ctx context.Context, operation providerops.Operation, item providerops.Item) error {
	_, err := s.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{
		Status: item.Status, ProviderReference: item.ProviderReference,
		ErrorCode: item.ErrorCode, ResultJSON: item.ResultJSON,
	}, s.now().UTC())
	return err
}
