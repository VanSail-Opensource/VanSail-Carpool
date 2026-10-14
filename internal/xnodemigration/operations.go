package xnodemigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	payload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

const OperationKind = "xnode_migration"

type OperationRequest struct {
	Action string `json:"action"`
	NodeID string `json:"nodeId,omitempty"`
	Digest string `json:"digest,omitempty"`
}
type operationRepository interface {
	CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	CompleteXNodeProviderOperation(context.Context, string, int, providerops.Completion, time.Time) (providerops.Operation, error)
}

func validRequest(request OperationRequest) bool {
	switch request.Action {
	case "import":
		return request.NodeID == "" && len(request.Digest) == 64
	case "cutover", "rollback":
		return request.Digest == "" && request.NodeID != "" && len(request.NodeID) <= 128
	default:
		return false
	}
}
func (s *Service) Queue(ctx context.Context, actor, key string, request OperationRequest) (model.OperationReceipt, error) {
	repo, ok := s.repo.(operationRepository)
	if !ok {
		return model.OperationReceipt{}, xnode.ErrUnavailable
	}
	if !validRequest(request) || key == "" || len(key) > 128 {
		return model.OperationReceipt{}, xnode.ErrInvalid
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	digest := sha256.Sum256(raw)
	sealed, err := s.vault.Encrypt("xnode.migration.operation."+actor+"."+key, string(raw))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	id := request.NodeID
	if id == "" {
		id = request.Digest
	}
	op, _, err := repo.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor, Kind: OperationKind, IdempotencyKey: key, RequestFingerprint: hex.EncodeToString(digest[:]), SealedTarget: sealed, Items: []providerops.ItemInput{{Key: id, TargetType: "xnode_migration", TargetID: id}}}, s.panel.Now())
	return op.Receipt, err
}
func (s *Service) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	if s.master == nil {
		return xnode.ErrUnavailable
	}
	return s.master.DeferOperation(job, func(ctx context.Context) error { return s.runOperation(ctx, operation, job) })
}
func (s *Service) runOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	repo, ok := s.repo.(operationRepository)
	if !ok {
		return xnode.ErrUnavailable
	}
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	sealed, err := payload.TargetID(job, "sealedTarget")
	if err != nil {
		return err
	}
	raw, err := s.vault.Decrypt("xnode.migration.operation."+operation.ActorUserID+"."+operation.IdempotencyKey, sealed)
	if err != nil {
		return err
	}
	var request OperationRequest
	if json.Unmarshal([]byte(raw), &request) != nil || !validRequest(request) {
		return xnode.ErrInvalid
	}
	operation, err = repo.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.panel.Now())
	if err != nil {
		return err
	}
	switch request.Action {
	case "import":
		_, err = s.Import(ctx, request.Digest)
	case "cutover":
		_, err = s.Cutover(ctx, request.NodeID)
	case "rollback":
		_, err = s.Rollback(ctx, request.NodeID)
	}
	completion := providerops.Completion{Status: providerops.StatusSucceeded}
	if err != nil {
		completion.Status, completion.ErrorCode = providerops.StatusPendingReview, "XNODE_MIGRATION_PENDING"
		if errors.Is(err, xnode.ErrInvalid) || errors.Is(err, xnode.ErrConflict) {
			completion.Status, completion.ErrorCode = providerops.StatusFailed, "XNODE_MIGRATION_INPUT_INVALID"
		}
	}
	// Receipts contain ownership only, never live node telemetry.
	result, err := json.Marshal(map[string]string{"action": request.Action, "nodeId": request.NodeID})
	if err != nil {
		return err
	}
	completion.ResultJSON = string(result)
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = repo.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, completion, s.panel.Now())
	return err
}
