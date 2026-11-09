package hourlybilling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type ActivationRequest struct {
	GroupID        string `json:"groupId"`
	PolicyVersion  string `json:"policyVersion"`
	OfferRevision  int64  `json:"offerRevision,string"`
	ActivationCode string `json:"activationCode,omitempty"`
}

type operationTarget struct {
	Generation     int64  `json:"accountGeneration,string"`
	GroupID        string `json:"groupId"`
	EnrollmentID   string `json:"enrollmentId,omitempty"`
	PolicyVersion  string `json:"policyVersion"`
	OfferRevision  int64  `json:"offerRevision,string"`
	ActivationCode string `json:"activationCode,omitempty"`
	Admin          bool   `json:"admin,omitempty"`
}

// Activate records durable intent before preparing credentials or debiting.
func (s *Service) Activate(ctx context.Context, user model.User, input ActivationRequest, key string) (model.OperationReceipt, error) {
	target := operationTarget{Generation: user.AccountGeneration, GroupID: strings.TrimSpace(input.GroupID), PolicyVersion: input.PolicyVersion, OfferRevision: input.OfferRevision, ActivationCode: input.ActivationCode}
	if target.GroupID == "" || len(target.GroupID) > 128 || len(target.PolicyVersion) != 64 || target.OfferRevision <= 0 || len(target.ActivationCode) > 1024 {
		return model.OperationReceipt{}, database.ErrConflict
	}
	return s.queue(ctx, user.ID, user.ID, ActivateOperation, key, target)
}

// Disable retains the current paid hour and applies the reviewed cooldown.
func (s *Service) Disable(ctx context.Context, user model.User, enrollmentID, version, key string) (model.OperationReceipt, error) {
	enrollment, err := s.store.HourlyEnrollment(ctx, enrollmentID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return model.OperationReceipt{}, err
	}
	if err != nil || enrollment.OwnerID != user.ID || enrollment.AccountGeneration != user.AccountGeneration {
		return model.OperationReceipt{}, database.ErrNotFound
	}
	if len(version) != 64 {
		return model.OperationReceipt{}, database.ErrConflict
	}
	return s.queue(ctx, user.ID, user.ID, DisableOperation, key, operationTarget{Generation: user.AccountGeneration, GroupID: enrollment.GroupID, EnrollmentID: enrollment.ID, PolicyVersion: version})
}

// queue records one actor/owner-scoped provider operation. Owner and actor are
// separate so administrator commands remain attributed to the operator.
func (s *Service) queue(ctx context.Context, actorID, ownerID, kind, key string, target operationTarget) (model.OperationReceipt, error) {
	key = strings.TrimSpace(key)
	if actorID == "" || ownerID == "" || target.Generation < 0 || key == "" || len(key) > 96 || s.vault == nil {
		return model.OperationReceipt{}, database.ErrConflict
	}
	// Generations have separate action namespaces; old receipts cannot enable a
	// rejoined account. Never persist the activation code in plaintext.
	encoded, err := json.Marshal(target)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	sealed, err := s.vault.Encrypt(operationScope(kind, ownerID), string(encoded))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint := targetFingerprint(kind, ownerID, target)
	scopedKey := operationActionKey(target.Generation, key)
	if receipt, found, err := s.store.ProviderOperationForActorKey(ctx, actorID, kind, scopedKey, fingerprint); found || err != nil {
		return receipt, err
	}
	operation, _, err := s.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actorID, OwnerUserID: ownerID,
		Kind: kind, IdempotencyKey: scopedKey, RequestFingerprint: fingerprint, SealedTarget: sealed,
		Items: []providerops.ItemInput{{Key: "hourly", TargetType: "hourly_squad", TargetID: target.GroupID}}}, s.now().UTC())
	return operation.Receipt, err
}

func targetFingerprint(kind, owner string, target operationTarget) string {
	target.ActivationCode = ""
	raw, _ := json.Marshal([]any{kind, owner, target})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func operationScope(kind, owner string) string { return "hourly.operation:" + kind + ":" + owner }

// operationActionKey namespaces an action by account generation so a rejoined
// account cannot replay an old receipt.
func operationActionKey(generation int64, key string) string {
	return strconv.FormatInt(generation, 10) + ":" + strings.TrimSpace(key)
}
