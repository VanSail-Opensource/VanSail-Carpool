package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"slices"
	"time"
)

const OperationKind = "xnode_command"

type CommandRequest struct {
	Force          bool   `json:"force,omitempty"`
	Name           string `json:"name"`
	JobID          string `json:"jobId,omitempty"`
	Version        string `json:"version,omitempty"`
	PeerEndpointID string `json:"peerEndpointId,omitempty"`
}
type operationRepository interface {
	CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	CompleteXNodeProviderOperation(context.Context, string, int, providerops.Completion, time.Time) (providerops.Operation, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
}

func validNodeCommand(request CommandRequest) bool {
	if request.Force && request.Name != "runtime.reload" {
		return false
	}
	switch request.Name {
	case "update.check", "update.rollback", "runtime.reload", "geodata.refresh":
		return request.JobID == "" && request.Version == "" && request.PeerEndpointID == ""
	case "update.install":
		return request.JobID == "" && request.PeerEndpointID == "" && len(request.Version) < 64
	case "ddns.run":
		return request.JobID != "" && len(request.JobID) <= 128 && request.Version == "" && request.PeerEndpointID == ""
	case "speedtest":
		return request.JobID == "" && request.Version == "" && request.PeerEndpointID != ""
	default:
		return false
	}
}
func (s *Service) QueueCommand(ctx context.Context, actor, node, key string, request CommandRequest) (model.OperationReceipt, error) {
	repository, ok := s.repo.(operationRepository)
	if !ok {
		return model.OperationReceipt{}, ErrUnavailable
	}
	if !validNodeCommand(request) || key == "" || len(key) > 128 {
		return model.OperationReceipt{}, ErrInvalid
	}
	if _, err := s.repo.XNodeNode(ctx, node); err != nil {
		return model.OperationReceipt{}, err
	}
	if request.Name == "runtime.reload" && s.master != nil && s.master.Connected(node) && !containsCapability(s.master.Capabilities(node), "incremental_reload_v1") {
		return model.OperationReceipt{}, &CapabilityError{Capability: "incremental_reload_v1"}
	}
	if (request.Name == "runtime.reload" || request.Name == "geodata.refresh") && s.master != nil && s.master.Connected(node) && !slices.Contains(s.master.Capabilities(node), "activation_commands_v1") {
		return model.OperationReceipt{}, &CapabilityError{Capability: "activation_commands_v1"}
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	hash := sha256.Sum256(append([]byte(node+"\n"), raw...))
	sealed, err := s.vault.Encrypt("xnode.command."+actor+"."+key, string(raw))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, _, err := repository.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor, Kind: OperationKind, IdempotencyKey: key, RequestFingerprint: hex.EncodeToString(hash[:]), SealedTarget: sealed, Items: []providerops.ItemInput{{Key: node, TargetType: "xnode", TargetID: node}}}, s.now().UTC())
	return operation.Receipt, err
}
func (s *Service) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	if operation.Receipt.Kind == FirewallExemptionOperation {
		return s.handleFirewallExemption(ctx, operation)
	}
	if s.master == nil {
		return ErrUnavailable
	}
	if operation.Receipt.Kind == RouteSpeedtestOperation {
		return s.handleRouteSpeedtest(ctx, operation, job)
	}
	if operation.Receipt.Kind == DIYOperationKind {
		return s.master.deferMemberOperation(job, func(ctx context.Context) error { return s.runDIYOperation(ctx, operation) })
	}
	if operation.Receipt.Kind == RoutingOperationKind {
		return s.handleRoutingOperation(operation, job)
	}
	if operation.Receipt.Kind == DDNSOperationKind {
		return s.master.deferOperation(job, 2*time.Minute, func(ctx context.Context) error { return s.runDDNSOperation(ctx, operation) })
	}
	return s.master.deferOperation(job, 12*time.Minute, func(ctx context.Context) error { return s.runProviderOperation(ctx, operation, job) })
}

func (s *Service) DeleteNode(ctx context.Context, id string) error {
	if s.master != nil && s.master.Connected(id) {
		return fmt.Errorf("%w: enrolled node must be drained before deletion", ErrConflict)
	}
	return s.repo.DeleteXNodeNode(ctx, id)
}
func (s *Service) DeleteEndpoint(ctx context.Context, id string) error {
	return s.repo.DeleteXNodeEndpoint(ctx, id)
}
