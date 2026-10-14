package emby

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type operationTarget struct {
	Password    string      `json:"password,omitempty"`
	Preferences Preferences `json:"preferences,omitempty"`
}

func (s *OperationService) queueSealed(ctx context.Context, actorID, ownerID, kind, key, targetType, targetID string,
	target operationTarget) (model.OperationReceipt, error) {
	return s.queue(ctx, actorID, ownerID, kind, key, targetType, targetID, target)
}

func (s *OperationService) queue(ctx context.Context, actorID, ownerID, kind, key, targetType, targetID string,
	target operationTarget) (model.OperationReceipt, error) {
	key = strings.TrimSpace(key)
	payload, fingerprint, err := s.targetEnvelope(kind, targetID, target)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if receipt, found, err := s.repository.ProviderOperationForActorKey(ctx, actorID, kind, key, fingerprint); found || err != nil {
		return receipt, err
	}
	sealed := ""
	if string(payload) != "{}" {
		sealed, err = s.secrets.Seal(operationSecretContext(ownerID, kind, fingerprint), payload)
		if err != nil {
			return model.OperationReceipt{}, err
		}
	}
	operation, _, err := s.repository.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: ownerID, Kind: kind, IdempotencyKey: key,
		RequestFingerprint: fingerprint, SealedTarget: sealed,
		Items: []providerops.ItemInput{{Key: "emby", TargetType: targetType, TargetID: targetID}},
	}, s.now().UTC())
	return operation.Receipt, err
}

func (s *OperationService) replayTarget(ctx context.Context, actorID, kind, key, targetID string,
	target operationTarget) (model.OperationReceipt, bool, error) {
	fingerprint, err := s.targetFingerprint(kind, targetID, target)
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	return s.repository.ProviderOperationForActorKey(ctx, actorID, kind, strings.TrimSpace(key), fingerprint)
}

func (s *OperationService) targetFingerprint(kind, targetID string, target operationTarget) (string, error) {
	_, fingerprint, err := s.targetEnvelope(kind, targetID, target)
	return fingerprint, err
}

func (s *OperationService) targetEnvelope(kind, targetID string,
	target operationTarget) ([]byte, string, error) {
	payload, err := json.Marshal(target)
	if err != nil {
		return nil, "", err
	}
	return payload, s.fingerprint(kind, targetID, payload), nil
}

func (s *OperationService) decodeTarget(job model.OutboxJob, operation providerops.Operation) (operationTarget, error) {
	sealed, err := jobpayload.TargetID(job, "sealedTarget")
	if err != nil {
		return operationTarget{}, err
	}
	plaintext, err := s.secrets.Open(operationSecretContext(operation.OwnerUserID, operation.Receipt.Kind,
		operation.RequestFingerprint), sealed)
	if err != nil {
		return operationTarget{}, err
	}
	defer zero(plaintext)
	var target operationTarget
	if json.Unmarshal(plaintext, &target) != nil {
		return operationTarget{}, errors.New("Emby operation target is invalid")
	}
	return target, nil
}

func (s *OperationService) fingerprint(kind, targetID string, payload []byte) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("emby-operation:v1\x00" + kind + "\x00" + strings.TrimSpace(targetID) + "\x00"))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func operationSecretContext(ownerID, kind, fingerprint string) string {
	return "emby-operation:" + ownerID + ":" + kind + ":" + fingerprint
}
