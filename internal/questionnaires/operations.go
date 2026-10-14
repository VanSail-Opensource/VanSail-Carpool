package questionnaires

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type operationStore interface {
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	BeginQuestionnaireSettlementOperation(context.Context, providerops.CreateInput, string, time.Time) (providerops.Operation, bool, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	SettleQuestionnaireImport(context.Context, string, time.Time) (SettlementReport, error)
}

// ConfirmImportOperation creates or replays a durable settlement receipt.
func (service *Service) ConfirmImportOperation(ctx context.Context, actorID, importID, key string) (model.OperationReceipt, error) {
	repository, ok := service.store.(operationStore)
	if !ok {
		return model.OperationReceipt{}, errors.New("durable questionnaire settlement is unavailable")
	}
	actorID, importID, key = strings.TrimSpace(actorID), strings.TrimSpace(importID), strings.TrimSpace(key)
	fingerprint := settlementFingerprint(importID)
	if receipt, found, err := repository.ProviderOperationForActorKey(ctx, actorID, providerops.KindQuestionnaireSettlement,
		key, fingerprint); found || err != nil {
		return receipt, err
	}
	operation, _, err := repository.BeginQuestionnaireSettlementOperation(ctx, providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: actorID, Kind: providerops.KindQuestionnaireSettlement,
		IdempotencyKey: key, RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "import", TargetType: "questionnaire_import", TargetID: importID}},
	}, importID, service.now().UTC())
	return operation.Receipt, err
}

func settlementFingerprint(importID string) string {
	digest := sha256.Sum256([]byte("questionnaire-settlement:v1:" + strings.TrimSpace(importID)))
	return hex.EncodeToString(digest[:])
}
