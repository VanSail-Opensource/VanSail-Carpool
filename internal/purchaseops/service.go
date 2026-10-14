package purchaseops

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

// Repository is the atomic local boundary for member purchase operations.
type Repository interface {
	MemberPurchaseFacts(context.Context, string) (PurchaseFacts, error)
	UserByID(context.Context, string) (model.User, error)
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	BeginTrafficReset(context.Context, providerops.CreateInput, string, time.Time) (providerops.Operation, bool, error)
	BeginMemberRefund(context.Context, providerops.CreateInput, string, time.Time) (providerops.Operation, bool, error)
	ProviderOperationForPrincipal(context.Context, string, string) (model.OperationReceipt, error)
}

// Remote provides live state and exact-state provider mutations through the queue.
type Remote interface {
	MemberOperationState(context.Context, string) (RemoteState, error)
	ResetTraffic(context.Context, string) error
	QuiesceMemberOperation(context.Context, string) error
	RestoreMemberOperation(context.Context, string, model.Purchase) error
	DefinitiveMutationFailure(error) bool
}

// Service owns quotes and idempotent durable command creation.
type Service struct {
	repository Repository
	remote     Remote
	now        func() time.Time
}

// NewService creates the member purchase operation service.
func NewService(repository Repository, remote Remote) *Service {
	return &Service{repository: repository, remote: remote, now: time.Now}
}

// TrafficResetQuote returns the immutable core-price cadence quote.
func (s *Service) TrafficResetQuote(ctx context.Context, userID, purchaseID string) (TrafficResetQuote, error) {
	facts, err := s.repository.MemberPurchaseFacts(ctx, purchaseID)
	if err != nil {
		return TrafficResetQuote{}, err
	}
	return QuoteTrafficReset(facts, userID, s.now().UTC())
}

// ResetTraffic atomically debits and queues one idempotent reset operation.
func (s *Service) ResetTraffic(ctx context.Context, userID, purchaseID, idempotencyKey string) (model.OperationReceipt, error) {
	fingerprint := operationFingerprint(OperationResetKind, purchaseID)
	if receipt, found, err := s.replay(ctx, userID, OperationResetKind, idempotencyKey, fingerprint); found || err != nil {
		return receipt, err
	}
	quote, err := s.TrafficResetQuote(ctx, userID, purchaseID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if !quote.Eligible {
		return model.OperationReceipt{}, ErrIneligible
	}
	operation, _, err := s.repository.BeginTrafficReset(ctx, command(userID, purchaseID, OperationResetKind, idempotencyKey, fingerprint), purchaseID, s.now().UTC())
	return operation.Receipt, err
}

// RefundQuote returns a live-usage, first-term refund quote.
func (s *Service) RefundQuote(ctx context.Context, userID, purchaseID string) (MemberRefundQuote, error) {
	facts, err := s.repository.MemberPurchaseFacts(ctx, purchaseID)
	if err != nil {
		return MemberRefundQuote{}, err
	}
	if facts.Purchase.UserID != userID {
		return MemberRefundQuote{}, ErrNotFound
	}
	local, err := QuoteMemberRefund(facts, userID, 0, s.now().UTC())
	if err != nil || !local.Eligible {
		return local, err
	}
	user, err := s.repository.UserByID(ctx, userID)
	if err != nil {
		return MemberRefundQuote{}, err
	}
	if user.RemnaUserID == nil {
		local.Eligible, local.ReasonCode = false, reason(ReasonRemoteUnlinked)
		return local, nil
	}
	state, err := s.remote.MemberOperationState(ctx, *user.RemnaUserID)
	if err != nil {
		return MemberRefundQuote{}, err
	}
	return QuoteMemberRefund(facts, userID, state.UsedTrafficBytes, s.now().UTC())
}

// RefundPurchase queues a live-validated first-term refund operation.
func (s *Service) RefundPurchase(ctx context.Context, userID, purchaseID, idempotencyKey string) (model.OperationReceipt, error) {
	fingerprint := operationFingerprint(OperationRefundKind, purchaseID)
	if receipt, found, err := s.replay(ctx, userID, OperationRefundKind, idempotencyKey, fingerprint); found || err != nil {
		return receipt, err
	}
	quote, err := s.RefundQuote(ctx, userID, purchaseID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if !quote.Eligible {
		return model.OperationReceipt{}, ErrIneligible
	}
	operation, _, err := s.repository.BeginMemberRefund(ctx, command(userID, purchaseID, OperationRefundKind, idempotencyKey, fingerprint), purchaseID, s.now().UTC())
	return operation.Receipt, err
}

// Operation returns one actor-or-owner-scoped durable receipt.
func (s *Service) Operation(ctx context.Context, userID, operationID string) (model.OperationReceipt, error) {
	return s.repository.ProviderOperationForPrincipal(ctx, operationID, userID)
}

func (s *Service) replay(ctx context.Context, userID, kind, key, fingerprint string) (model.OperationReceipt, bool, error) {
	if strings.TrimSpace(key) == "" || len(strings.TrimSpace(key)) > 128 {
		return model.OperationReceipt{}, false, errors.New("invalid idempotency key")
	}
	return s.repository.ProviderOperationForActorKey(ctx, userID, kind, key, fingerprint)
}

func command(userID, purchaseID, kind, key, fingerprint string) providerops.CreateInput {
	return providerops.CreateInput{ActorUserID: userID, OwnerUserID: userID, Kind: kind, IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: purchaseID}}}
}

func operationFingerprint(kind, purchaseID string) string {
	digest := sha256.Sum256([]byte("member-operation:v1:" + kind + ":" + strings.TrimSpace(purchaseID)))
	return hex.EncodeToString(digest[:])
}
