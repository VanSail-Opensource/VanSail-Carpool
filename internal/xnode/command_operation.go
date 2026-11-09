package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	payload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	jobqueue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"time"
)

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
	var intent queuedNodeCommand
	if json.Unmarshal([]byte(raw), &intent) != nil || !validNodeCommand(intent.CommandRequest) {
		return ErrInvalid
	}
	request := intent.CommandRequest
	if request.Name == "node.remove" {
		return s.runNodeRemoval(ctx, operation, items[0].TargetID)
	}
	preflight, stopPreflight := context.WithTimeout(ctx, 30*time.Second)
	args, err := s.operationArguments(preflight, items[0].TargetID, operation, intent)
	if err == nil {
		operation, err = s.prepareUpdate(preflight, items[0].TargetID, operation, request, args)
	}
	stopPreflight()
	if errors.Is(err, errCertificateEvidence) {
		return s.completeCertificateReview(ctx, repository, operation, items[0].TargetID, intent)
	}
	if err != nil && errors.Is(err, ErrUpdateEvidence) {
		facts, encodeErr := updateOperationResult(operation, request, items[0].TargetID, nil)
		if encodeErr != nil {
			return encodeErr
		}
		finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, err = repository.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "XNODE_UPDATE_UNCONFIRMED", ResultJSON: facts}, s.now().UTC())
		return err
	}
	if err != nil {
		var capability *CapabilityError
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) || errors.As(err, &capability) {
			code := "XNODE_COMMAND_INPUT_INVALID"
			if capability != nil {
				code = "XNODE_UPGRADE_REQUIRED"
			}
			finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			facts := "{}"
			if request.Name == "certificates.run" {
				facts, err = certificateOperationResult(request, items[0].TargetID, nil, intent.AgentID)
				if err != nil {
					return err
				}
			}
			_, finishErr := repository.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, providerops.Completion{Status: providerops.StatusFailed, ErrorCode: code, ResultJSON: facts}, s.now().UTC())
			return finishErr
		}
		return err
	}
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	operation, err = repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
	if err != nil {
		return err
	}
	child, cancel := context.WithTimeout(ctx, commandDeadline(request.Name)+30*time.Second)
	defer cancel()
	result, commandErr := s.executeNodeCommand(child, items[0].TargetID, operation, request, args, intent.AgentID)
	if errors.Is(commandErr, jobqueue.ErrBusy) {
		if repo, ok := s.repo.(interface {
			DeferXNodeCommand(context.Context, string, int, time.Time) error
		}); ok {
			if err := repo.DeferXNodeCommand(ctx, operation.Receipt.ID, operation.Attempts, s.now().UTC()); err != nil {
				return err
			}
			return fmt.Errorf("XNODE_COMMAND_BUSY: %w", jobqueue.ErrBusy)
		}
	}
	completion := providerops.Completion{Status: providerops.StatusSucceeded, ProviderReference: operation.Receipt.ID, ResultJSON: `{}`}
	if isUpdateCommand(request.Name) {
		completion.ResultJSON, err = updateOperationResult(operation, request, items[0].TargetID, result)
		if err != nil {
			return err
		}
	}
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
		if errors.Is(commandErr, ErrUpdateEvidence) {
			completion.ErrorCode = "XNODE_UPDATE_UNCONFIRMED"
		}
		if errors.Is(commandErr, ErrUpdateNotReady) {
			completion.ErrorCode = "XNODE_UPDATE_READINESS_PENDING"
		}
	} else if !isUpdateCommand(request.Name) {
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
	if request.Name == "certificates.run" {
		completion.ResultJSON, err = certificateOperationResult(request, items[0].TargetID, result, intent.AgentID)
		if err != nil {
			return err
		}
	}
	finish, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelFinish()
	_, err = repository.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, completion, s.now().UTC())
	return err
}
