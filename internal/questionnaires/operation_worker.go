package questionnaires

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// HandleProviderOperation applies a local settlement transaction idempotently.
func (service *Service) HandleProviderOperation(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	if operation.Receipt.Kind != providerops.KindQuestionnaireSettlement {
		return errors.New("unsupported questionnaire operation")
	}
	repository, ok := service.store.(operationStore)
	if !ok {
		return errors.New("durable questionnaire settlement is unavailable")
	}
	items, err := repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "questionnaire_import" {
		if err == nil {
			err = errors.New("questionnaire settlement has an invalid target")
		}
		return err
	}
	item := items[0]
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, service.now().UTC())
		if err != nil {
			return err
		}
	}
	if item.Status == providerops.StatusQueued {
		item, err = repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, service.now().UTC())
		if err != nil {
			return err
		}
	}
	report, err := repository.SettleQuestionnaireImport(ctx, item.TargetID, service.now().UTC())
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"importId": report.ImportID, "rewardedCount": report.RewardedCount})
	if err != nil {
		return err
	}
	completion := providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: string(result)}
	if _, err := repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, service.now().UTC()); err != nil {
		return err
	}
	_, err = repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, service.now().UTC())
	return err
}

var _ providerops.Handler = (*Service)(nil)
