package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type revokeOperationRepository interface {
	CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	UserByID(context.Context, string) (model.User, error)
}

type revokeTarget struct {
	PreviousHash string `json:"previousHash"`
}

// QueueSubscriptionRevoke records one credential rotation before provider mutation.
func (s *Service) QueueSubscriptionRevoke(ctx context.Context, user model.User, key string) (model.OperationReceipt, error) {
	repository, ok := s.repository.(revokeOperationRepository)
	if !ok || user.RemnaUserID == nil {
		return model.OperationReceipt{}, errors.New("durable subscription revocation is unavailable")
	}
	fingerprint := revokeFingerprint(user.ID)
	if receipt, found, err := repository.ProviderOperationForActorKey(ctx, user.ID, providerops.KindSubscriptionRevoke,
		strings.TrimSpace(key), fingerprint); found || err != nil {
		return receipt, err
	}
	current, err := s.remnawave.Dashboard(ctx, *user.RemnaUserID)
	if err != nil || strings.TrimSpace(current.SubscriptionURL) == "" {
		return model.OperationReceipt{}, errors.New("load current subscription credential")
	}
	target, err := json.Marshal(revokeTarget{PreviousHash: subscriptionHash(current.SubscriptionURL)})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, _, err := repository.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: user.ID, OwnerUserID: user.ID, Kind: providerops.KindSubscriptionRevoke,
		IdempotencyKey: strings.TrimSpace(key), RequestFingerprint: fingerprint, SealedTarget: string(target),
		Items: []providerops.ItemInput{{Key: "subscription", TargetType: "user", TargetID: user.ID}},
	}, s.now().UTC())
	return operation.Receipt, err
}

func revokeFingerprint(userID string) string {
	return subscriptionHash("subscription-revoke:v1:" + strings.TrimSpace(userID))
}

func subscriptionHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
