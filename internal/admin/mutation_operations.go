package admin

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

type mutationOperationRepository interface {
	CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	CompleteOutboxRetryOperation(context.Context, string, string, string, time.Time) error
	PaymentOrderByID(context.Context, string) (model.PaymentOrder, error)
	RefundPayment(context.Context, *string, string, string, time.Time) (model.PaymentOrder, error)
}

type refundTarget struct {
	Reason string `json:"reason"`
}

// QueueJobRetry creates or replays an audited local retry command.
func (s *Service) QueueJobRetry(ctx context.Context, actorID, jobID, key string) (model.OperationReceipt, error) {
	return s.queueMutation(ctx, actorID, actorID, providerops.KindOutboxRetry, key, "outbox_job", jobID, nil)
}

// QueueMaintenance creates or replays an audited manual maintenance command.
func (s *Service) QueueMaintenance(ctx context.Context, actorID, key string) (model.OperationReceipt, error) {
	return s.queueMutation(ctx, actorID, actorID, providerops.KindAdminMaintenance, key, "maintenance", "retention", nil)
}

// QueueRefund creates or replays a provider-aware payment refund command.
func (s *Service) QueueRefund(ctx context.Context, actorID, orderID, reason, key string) (model.OperationReceipt, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return model.OperationReceipt{}, errors.New("refund reason is required")
	}
	payload, err := json.Marshal(refundTarget{Reason: reason})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	repository, ok := s.repository.(mutationOperationRepository)
	if !ok {
		return model.OperationReceipt{}, errors.New("durable administrator operations are unavailable")
	}
	fingerprint := mutationFingerprint(providerops.KindPaymentRefund, orderID, payload)
	if receipt, found, err := repository.ProviderOperationForActorKey(ctx, actorID, providerops.KindPaymentRefund,
		strings.TrimSpace(key), fingerprint); found || err != nil {
		return receipt, err
	}
	order, err := s.repository.PaymentOrderByID(ctx, orderID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.queueMutation(ctx, actorID, order.UserID, providerops.KindPaymentRefund, key, "payment_order", orderID, payload)
}

func (s *Service) queueMutation(ctx context.Context, actorID, ownerID, kind, key, targetType, targetID string,
	target []byte) (model.OperationReceipt, error) {
	repository, ok := s.repository.(mutationOperationRepository)
	if !ok {
		return model.OperationReceipt{}, errors.New("durable administrator operations are unavailable")
	}
	fingerprint := mutationFingerprint(kind, targetID, target)
	if receipt, found, err := repository.ProviderOperationForActorKey(ctx, actorID, kind, strings.TrimSpace(key), fingerprint); found || err != nil {
		return receipt, err
	}
	operation, replayed, err := repository.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: ownerID, Kind: kind, IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, SealedTarget: string(target),
		Items: []providerops.ItemInput{{Key: "target", TargetType: targetType, TargetID: targetID}},
	}, s.now().UTC())
	if err != nil || replayed {
		return operation.Receipt, err
	}
	if err := s.audit(ctx, actorID, kind+".queue", targetType, targetID, map[string]string{"operationId": operation.Receipt.ID}); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}

func mutationFingerprint(kind, targetID string, target []byte) string {
	digest := sha256.Sum256(append([]byte(kind+"\x00"+strings.TrimSpace(targetID)+"\x00"), target...))
	return hex.EncodeToString(digest[:])
}
