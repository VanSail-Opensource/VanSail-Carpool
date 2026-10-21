package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	payload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"slices"
	"time"
)

const OperationKind = "xnode_command"

type CommandRequest struct {
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
	if operation.Receipt.Kind == RoutingOperationKind || operation.Receipt.Kind == DIYOperationKind {
		return s.handleRoutingOperation(operation, job)
	}
	return s.master.deferOperation(job, 12*time.Minute, func(ctx context.Context) error { return s.runProviderOperation(ctx, operation, job) })
}

func (s *Service) runProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	repository, ok := s.repo.(operationRepository)
	if !ok || s.master == nil {
		return ErrUnavailable
	}
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	items, err := repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].TargetType != "xnode" {
		return ErrInvalid
	}
	sealed, err := payload.TargetID(job, "sealedTarget")
	if err != nil {
		return err
	}
	raw, err := s.vault.Decrypt("xnode.command."+operation.ActorUserID+"."+operation.IdempotencyKey, sealed)
	if err != nil {
		return err
	}
	var request CommandRequest
	if json.Unmarshal([]byte(raw), &request) != nil || !validNodeCommand(request) {
		return ErrInvalid
	}
	preflight, stopPreflight := context.WithTimeout(ctx, 30*time.Second)
	args, err := s.commandArguments(preflight, items[0].TargetID, request)
	stopPreflight()
	if err != nil {
		var capability *CapabilityError
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) || errors.As(err, &capability) {
			code := "XNODE_COMMAND_INPUT_INVALID"
			if capability != nil {
				code = "XNODE_UPGRADE_REQUIRED"
			}
			finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_, finishErr := repository.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, providerops.Completion{Status: providerops.StatusFailed, ErrorCode: code, ResultJSON: "{}"}, s.now().UTC())
			return finishErr
		}
		return err
	}
	operation, err = repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
	if err != nil {
		return err
	}
	child, cancel := context.WithTimeout(ctx, commandDeadline(request.Name)+30*time.Second)
	defer cancel()
	result, commandErr := s.executeNodeCommand(child, items[0].TargetID, operation, request, args)
	completion := providerops.Completion{Status: providerops.StatusSucceeded, ProviderReference: operation.Receipt.ID, ResultJSON: `{}`}
	if commandErr != nil {
		completion.Status = providerops.StatusPendingReview
		completion.ErrorCode = "XNODE_COMMAND_OUTCOME_UNKNOWN"
		var outcome *CommandError
		if errors.As(commandErr, &outcome) {
			switch outcome.Outcome {
			case CommandUndelivered:
				completion.Status, completion.ErrorCode = providerops.StatusFailed, "XNODE_COMMAND_UNDELIVERED"
			case CommandRejected:
				completion.Status, completion.ErrorCode = providerops.StatusFailed, "XNODE_COMMAND_REJECTED"
			case CommandFailed:
				completion.Status, completion.ErrorCode = providerops.StatusFailed, "XNODE_COMMAND_FAILED"
			}
		}
	} else {
		value := map[string]any{"command": request.Name, "nodeId": items[0].TargetID}
		if request.Name == "speedtest" {
			var data map[string]json.RawMessage
			if json.Unmarshal(result, &data) != nil {
				return ErrInvalid
			}
			for _, key := range []string{"request_latency_millis", "download_bytes", "upload_bytes", "download_mbps", "upload_mbps", "duration_millis"} {
				if v, ok := data[key]; ok {
					value[key] = v
				}
			}
		}
		stored, err := json.Marshal(value)
		if err != nil {
			return err
		}
		completion.ResultJSON = string(stored)
	}
	finish, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelFinish()
	_, err = repository.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, completion, s.now().UTC())
	return err
}
func (s *Service) DeleteNode(ctx context.Context, id string) error {
	if s.master != nil && s.master.Connected(id) {
		return fmt.Errorf("%w: enrolled node must be drained before deletion", ErrConflict)
	}
	return s.repo.DeleteXNodeNode(ctx, id)
}
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	return s.repo.DeleteXNodeGroup(ctx, id)
}
func (s *Service) DeleteEndpoint(ctx context.Context, id string) error {
	return s.repo.DeleteXNodeEndpoint(ctx, id)
}
