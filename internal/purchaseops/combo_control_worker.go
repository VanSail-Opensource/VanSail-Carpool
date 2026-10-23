package purchaseops

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// ComboControlWorkerRepository exposes only receipt transitions and target checks.
type ComboControlWorkerRepository interface {
	PurchaseByID(context.Context, string) (model.Purchase, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
}

// EntitlementSynchronizer uses the existing identity and reset phase executor.
type EntitlementSynchronizer interface {
	HandleOutbox(context.Context, model.OutboxJob) error
}

// ComboControlWorker bridges member receipts to the established entitlement executor.
type ComboControlWorker struct {
	repository ComboControlWorkerRepository
	sync       EntitlementSynchronizer
	onSynced   func(string)
}

// NewComboControlWorker does not create another queue or background worker.
func NewComboControlWorker(repository ComboControlWorkerRepository, synchronizer EntitlementSynchronizer, onSynced func(string)) *ComboControlWorker {
	return &ComboControlWorker{repository: repository, sync: synchronizer, onSynced: onSynced}
}

// HandleProviderOperation resumes durable activation phases and completes the receipt.
func (w *ComboControlWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	now := time.Now().UTC()
	if operation.Receipt.Status == "queued" {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now)
		if err != nil {
			return err
		}
	}
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].TargetType != "purchase" {
		return errors.New("combo control has an invalid target")
	}
	item := items[0]
	if item.Status == providerops.StatusSucceeded {
		_, err = w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: "{}"}, now)
		return err
	}
	if item.Status == providerops.StatusQueued {
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, now)
		if err != nil {
			return err
		}
	}
	target, err := w.repository.PurchaseByID(ctx, item.TargetID)
	if err != nil {
		return err
	}
	if target.UserID != operation.OwnerUserID {
		return errors.New("combo control owner mismatch")
	}
	kind, field, id := "xnode_sync_user", "userId", operation.OwnerUserID
	if operation.Receipt.Kind == OperationEarlyActivation {
		if target.Status == "cancelled" || target.Status == "expired" {
			return w.finish(ctx, operation, item, providerops.StatusFailed, "SUPERSEDED")
		}
		kind, field, id = "xnode_apply_entitlement", "purchaseId", item.TargetID
	} else if operation.Receipt.Kind != OperationSquadSwitch {
		return errors.New("unsupported combo control")
	}
	payload, err := json.Marshal(map[string]string{field: id})
	if err != nil {
		return err
	}
	job.Kind, job.Payload = kind, string(payload)
	if err := w.sync.HandleOutbox(ctx, job); err != nil {
		if job.Attempts >= 10 {
			return errors.Join(err, w.finish(ctx, operation, item, providerops.StatusFailed, "ENTITLEMENT_SYNC_FAILED"))
		}
		return err
	}
	if operation.Receipt.Kind == OperationEarlyActivation {
		applied, err := w.repository.PurchaseByID(ctx, item.TargetID)
		if err != nil {
			return err
		}
		if applied.Status != "active" {
			return w.finish(ctx, operation, item, providerops.StatusFailed, "ACTIVATION_UNAVAILABLE")
		}
	}
	if w.onSynced != nil {
		w.onSynced(operation.OwnerUserID)
	}
	return w.finish(ctx, operation, item, providerops.StatusSucceeded, "")
}

func (w *ComboControlWorker) finish(ctx context.Context, operation providerops.Operation, item providerops.Item, status providerops.Status, code string) error {
	now := time.Now().UTC()
	completion := providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}
	if _, err := w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, now); err != nil {
		return err
	}
	_, err := w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, now)
	return err
}
