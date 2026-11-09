package hourlybilling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	payload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	queue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func wait(reason string, err error) error {
	return &queue.WaitError{Reason: reason, Cause: err, After: 5 * time.Second}
}

// HandleProviderOperation resumes a single durable intent; uncertain payment or
// native results remain pending without consuming the queue's retry budget.
func (s *Service) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	target, err := s.decode(ctx, operation, job)
	targetErr := err
	if err != nil && !errors.Is(err, database.ErrAccountUnavailable) {
		return wait("HOURLY_TARGET_PENDING", err)
	}
	items, err := s.store.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return wait("HOURLY_RECEIPT_PENDING", err)
	}
	if len(items) != 1 || items[0].Key != "hourly" || items[0].TargetType != "hourly_squad" || items[0].TargetID != target.GroupID {
		return wait("HOURLY_TARGET_MISMATCH", database.ErrConflict)
	}
	item := items[0]
	if providerops.Terminal(item.Status) {
		return s.finish(ctx, operation, item, item.Status, item.ErrorCode)
	}
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = s.store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
	}
	if err == nil && item.Status == providerops.StatusQueued {
		item, err = s.store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, s.now().UTC())
	}
	if err != nil {
		return wait("HOURLY_RECEIPT_PENDING", err)
	}
	if errors.Is(targetErr, database.ErrAccountUnavailable) {
		return s.finish(ctx, operation, item, providerops.StatusFailed, "ACCOUNT_GENERATION_SUPERSEDED")
	}
	switch operation.Receipt.Kind {
	case ActivateOperation:
		return s.activateOperation(ctx, operation, item, target)
	case DisableOperation:
		if target.Admin {
			return s.adminDisableOperation(ctx, operation, item, target)
		}
		return s.disableOperation(ctx, operation, item, target)
	default:
		return s.finish(ctx, operation, item, providerops.StatusFailed, "HOURLY_OPERATION_INVALID")
	}
}

func (s *Service) decode(ctx context.Context, op providerops.Operation, job model.OutboxJob) (operationTarget, error) {
	var target operationTarget
	if s.vault == nil || op.OwnerUserID == "" {
		return target, database.ErrConflict
	}
	sealed, err := payload.TargetID(job, "sealedTarget")
	if err != nil {
		return target, err
	}
	raw, err := s.vault.Decrypt(operationScope(op.Receipt.Kind, op.OwnerUserID), sealed)
	if err != nil {
		return target, err
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&target) != nil || decoder.Decode(new(any)) != io.EOF || target.Generation < 0 || target.GroupID == "" || targetFingerprint(op.Receipt.Kind, op.OwnerUserID, target) != op.RequestFingerprint {
		return target, database.ErrConflict
	}
	// Member commands are owner-authored. Only the narrow administrator disable
	// may carry a distinct actor, and its sealed target must say so.
	if op.ActorUserID != op.OwnerUserID && !(op.Receipt.Kind == DisableOperation && target.Admin) {
		return target, database.ErrConflict
	}
	user, err := s.store.UserByID(ctx, op.OwnerUserID)
	if errors.Is(err, database.ErrNotFound) {
		err = database.ErrAccountUnavailable
	}
	if err == nil && (user.AccountGeneration != target.Generation || user.AccountStatus != "active") {
		err = database.ErrAccountUnavailable
	}
	return target, err
}

func (s *Service) finish(ctx context.Context, op providerops.Operation, item providerops.Item, status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}
	if item.Status == providerops.StatusProcessing {
		if _, err := s.store.CompleteProviderOperationItem(ctx, op.Receipt.ID, item.Key, completion, s.now().UTC()); err != nil {
			return wait("HOURLY_RECEIPT_PENDING", err)
		}
	}
	_, err := s.store.CompleteProviderOperation(ctx, op.Receipt.ID, completion, s.now().UTC())
	if err != nil {
		return wait("HOURLY_RECEIPT_PENDING", err)
	}
	return nil
}

func businessFailure(err error) bool {
	for _, expected := range []error{database.ErrConflict, database.ErrNotFound, database.ErrAccountUnavailable,
		database.ErrInsufficientBalance, database.ErrStockUnavailable, database.ErrActivationCodeRequired,
		database.ErrActivationCodeInvalid, database.ErrActivationCodeExtra, database.ErrHourlySalesDisabled,
		database.ErrHourlyCooldown, database.ErrHourlyLegacyConflict, database.ErrHourlyCapacity, model.ErrHourlyPolicyInvalid} {
		if errors.Is(err, expected) {
			return true
		}
	}
	return false
}
