package emby

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type operationRepository interface {
	CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
	BeginEmbySetupOperation(context.Context, providerops.CreateInput, QueueSetupInput, time.Time) (providerops.Operation, bool, error)
	BeginEmbyProvisionRetryOperation(context.Context, providerops.CreateInput, string, time.Time) (providerops.Operation, bool, error)
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
}

// OperationService owns idempotent Emby command creation and reconciliation.
type OperationService struct {
	core       *Service
	repository operationRepository
	secrets    SecretBox
	key        []byte
	now        func() time.Time
}

// NewOperationService creates the durable Emby command boundary.
func NewOperationService(core *Service, repository operationRepository, secrets SecretBox, key []byte) (*OperationService, error) {
	if core == nil || repository == nil || secrets == nil || len(key) < 32 {
		return nil, errors.New("Emby operation dependencies are incomplete")
	}
	return &OperationService{core: core, repository: repository, secrets: secrets, key: append([]byte(nil), key...), now: time.Now}, nil
}

// QueueSetup creates or replays one priced provisioning command.
func (s *OperationService) QueueSetup(ctx context.Context, userID, password string, preferences Preferences, key string) (model.OperationReceipt, error) {
	if strings.TrimSpace(userID) == "" || len(password) == 0 || len(password) > maxPasswordBytes {
		return model.OperationReceipt{}, ErrInvalidSetup
	}
	preferences = normalizePreferences(preferences)
	target := operationTarget{Password: password, Preferences: preferences}
	fingerprint, err := s.targetFingerprint(providerops.KindEmbySetup, userID, target)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	key = strings.TrimSpace(key)
	if receipt, found, err := s.repository.ProviderOperationForActorKey(ctx, userID, providerops.KindEmbySetup,
		key, fingerprint); found || err != nil {
		return receipt, err
	}
	setup, err := s.core.prepareSetup(ctx, userID, password, preferences)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, _, err := s.repository.BeginEmbySetupOperation(ctx, providerops.CreateInput{
		ActorUserID: userID, OwnerUserID: userID, Kind: providerops.KindEmbySetup,
		IdempotencyKey: key, RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "emby", TargetType: "emby_user", TargetID: userID}},
	}, setup, s.now().UTC())
	return operation.Receipt, err
}

// QueuePreferences creates or replays one exact policy replacement.
func (s *OperationService) QueuePreferences(ctx context.Context, userID string, preferences Preferences, key string) (model.OperationReceipt, error) {
	preferences = normalizePreferences(preferences)
	target := operationTarget{Preferences: preferences}
	if receipt, found, err := s.replayTarget(ctx, userID, providerops.KindEmbyPreferences, key, userID, target); found || err != nil {
		return receipt, err
	}
	if err := s.requireActive(ctx, userID); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := s.core.validatePreferences(ctx, preferences); err != nil {
		return model.OperationReceipt{}, err
	}
	return s.queueSealed(ctx, userID, userID, providerops.KindEmbyPreferences, key, "emby_user", userID, target)
}

// QueuePassword creates or replays one write-only password replacement.
func (s *OperationService) QueuePassword(ctx context.Context, userID, password, key string) (model.OperationReceipt, error) {
	if len(password) == 0 || len(password) > maxPasswordBytes {
		return model.OperationReceipt{}, ErrInvalidSetup
	}
	target := operationTarget{Password: password}
	if receipt, found, err := s.replayTarget(ctx, userID, providerops.KindEmbyPassword, key, userID, target); found || err != nil {
		return receipt, err
	}
	if err := s.requireActive(ctx, userID); err != nil {
		return model.OperationReceipt{}, err
	}
	return s.queueSealed(ctx, userID, userID, providerops.KindEmbyPassword, key, "emby_user", userID, target)
}

// QueueProvisionRetry records an administrator-owned explicit retry.
func (s *OperationService) QueueProvisionRetry(ctx context.Context, actorID, accountID, key string) (model.OperationReceipt, error) {
	fingerprint, err := s.targetFingerprint(providerops.KindEmbyProvisionRetry, accountID, operationTarget{})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	key = strings.TrimSpace(key)
	if receipt, found, err := s.repository.ProviderOperationForActorKey(ctx, actorID, providerops.KindEmbyProvisionRetry,
		key, fingerprint); found || err != nil {
		return receipt, err
	}
	record, err := s.core.repository.EmbyProvisioningByID(ctx, accountID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, _, err := s.repository.BeginEmbyProvisionRetryOperation(ctx, providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: record.UserID, Kind: providerops.KindEmbyProvisionRetry,
		IdempotencyKey: key, RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "emby", TargetType: "emby_account", TargetID: accountID}},
	}, accountID, s.now().UTC())
	return operation.Receipt, err
}

func (s *OperationService) requireActive(ctx context.Context, userID string) error {
	account, err := s.core.repository.EmbyAccountForUser(ctx, userID)
	if err != nil {
		return err
	}
	if account.Status != StatusActive || account.RemoteUserID == "" {
		return ErrRemoteAccountMissing
	}
	return nil
}
