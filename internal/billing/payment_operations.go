package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const (
	// OperationCreateKind creates one provider checkout from a durable order.
	OperationCreateKind = "payment_create"
	// OperationCancelKind cancels one provider checkout without affecting late settlement.
	OperationCancelKind = "payment_cancel"
)

// PaymentOperation identifies the durable command and its payment order.
type PaymentOperation struct {
	Operation      model.OperationReceipt `json:"operation"`
	PaymentOrderID string                 `json:"paymentOrderId"`
}

type paymentOperationRepository interface {
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginPaymentOrderOperation(context.Context, model.PaymentOrder, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
	BeginPaymentCancellationOperation(context.Context, string, string, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
}

// QueueOrder atomically persists a priced order and its provider command.
func (s *Service) QueueOrder(ctx context.Context, user model.User, methodID string, txbMinor int64,
	idempotencyKey string) (PaymentOperation, error) {
	repository, ok := s.repository.(paymentOperationRepository)
	if !ok {
		return PaymentOperation{}, errors.New("durable payment operations are unavailable")
	}
	key := strings.TrimSpace(idempotencyKey)
	fingerprint := paymentFingerprint(OperationCreateKind, strings.ToLower(strings.TrimSpace(methodID)), strconv.FormatInt(txbMinor, 10))
	if replay, found, err := repository.ProviderOperationForActorKey(ctx, user.ID, OperationCreateKind, key, fingerprint); found || err != nil {
		return paymentOperationForReceipt(ctx, repository, replay, err)
	}
	order, err := s.prepareOrder(ctx, user, methodID, txbMinor)
	if err != nil {
		return PaymentOperation{}, err
	}
	order.ID, err = ids.New()
	if err != nil {
		return PaymentOperation{}, err
	}
	input := paymentCommand(user.ID, OperationCreateKind, key, fingerprint, order.ID)
	operation, replayed, err := repository.BeginPaymentOrderOperation(ctx, order, input, s.now().UTC())
	if err != nil {
		return PaymentOperation{}, err
	}
	if replayed {
		return paymentOperationForReceipt(ctx, repository, operation.Receipt, nil)
	}
	return PaymentOperation{Operation: operation.Receipt, PaymentOrderID: order.ID}, nil
}

// QueueCancellation records local cancellation and queues any provider mutation atomically.
func (s *Service) QueueCancellation(ctx context.Context, orderID, userID, idempotencyKey string) (PaymentOperation, error) {
	repository, ok := s.repository.(paymentOperationRepository)
	if !ok {
		return PaymentOperation{}, errors.New("durable payment operations are unavailable")
	}
	key := strings.TrimSpace(idempotencyKey)
	fingerprint := paymentFingerprint(OperationCancelKind, strings.TrimSpace(orderID))
	if replay, found, err := repository.ProviderOperationForActorKey(ctx, userID, OperationCancelKind, key, fingerprint); found || err != nil {
		return paymentOperationForReceipt(ctx, repository, replay, err)
	}
	if _, err := s.repository.PaymentOrderForUser(ctx, orderID, userID); err != nil {
		return PaymentOperation{}, err
	}
	input := paymentCommand(userID, OperationCancelKind, key, fingerprint, orderID)
	operation, replayed, err := repository.BeginPaymentCancellationOperation(ctx, orderID, userID, input, s.now().UTC())
	if err != nil {
		return PaymentOperation{}, err
	}
	if replayed {
		return paymentOperationForReceipt(ctx, repository, operation.Receipt, nil)
	}
	return PaymentOperation{Operation: operation.Receipt, PaymentOrderID: orderID}, nil
}

func paymentCommand(userID, kind, key, fingerprint, orderID string) providerops.CreateInput {
	return providerops.CreateInput{
		ActorUserID: userID, OwnerUserID: userID, Kind: kind, IdempotencyKey: key,
		RequestFingerprint: fingerprint,
		Items:              []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: orderID}},
	}
}

func paymentOperationForReceipt(ctx context.Context, repository paymentOperationRepository,
	receipt model.OperationReceipt, err error) (PaymentOperation, error) {
	if err != nil {
		return PaymentOperation{}, err
	}
	items, err := repository.ProviderOperationItems(ctx, receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "payment_order" {
		if err == nil {
			err = errors.New("payment operation has an invalid target")
		}
		return PaymentOperation{}, err
	}
	return PaymentOperation{Operation: receipt, PaymentOrderID: items[0].TargetID}, nil
}

func paymentFingerprint(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
