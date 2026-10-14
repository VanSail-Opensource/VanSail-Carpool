package connections

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// UnblockWorker removes one encrypted active block through the provider queue.
type UnblockWorker struct {
	repository BlockWorkerRepository
	remote     BlockRemote
	secrets    SecretBox
	now        func() time.Time
}

func NewUnblockWorker(repository BlockWorkerRepository, remote BlockRemote, secrets SecretBox) *UnblockWorker {
	return &UnblockWorker{repository: repository, remote: remote, secrets: secrets, now: time.Now}
}

func (w *UnblockWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	operation, item, block, ip, err := w.prepare(ctx, operation)
	if err != nil {
		return err
	}
	if item.Status == providerops.StatusProcessing {
		return w.finish(ctx, operation, item, block, providerops.StatusPendingReview, "UNBLOCK_OUTCOME_AMBIGUOUS")
	}
	if item.Status != providerops.StatusQueued {
		return nil
	}
	item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	if err != nil {
		return err
	}
	scoped, callErr := scopedControl(ctx, w.remote, block.UserID, ip, block.NodeUUID, false, 0, true)
	if !scoped {
		callErr = w.remote.UnblockIP(ctx, ip, block.NodeUUID)
	}
	if callErr != nil {
		status, code := providerops.StatusPendingReview, "UNBLOCK_OUTCOME_AMBIGUOUS"
		if w.remote.DefinitiveMutationFailure(callErr) {
			status, code = providerops.StatusFailed, "UNBLOCK_REJECTED"
		}
		return w.finish(ctx, operation, item, block, status, code)
	}
	return w.finish(ctx, operation, item, block, providerops.StatusSucceeded, "")
}

func (w *UnblockWorker) prepare(ctx context.Context, operation providerops.Operation) (providerops.Operation, providerops.Item, IPBlockRecord, string, error) {
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", err
		}
	}
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "connection_ip_hmac" {
		return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", errors.New("connection unblock has an invalid target")
	}
	block, err := w.repository.ConnectionIPBlockByOperation(ctx, operation.Receipt.ID)
	if err != nil {
		return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", err
	}
	plaintext, err := w.secrets.Open(ipBlockSecretContext(block.UserID, block.NodeUUID, block.IPDigest), block.SealedIP)
	if err != nil || items[0].TargetID != block.IPDigest {
		return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", errors.New("connection unblock target could not be verified")
	}
	return operation, items[0], block, string(plaintext), nil
}

func (w *UnblockWorker) finish(ctx context.Context, operation providerops.Operation, item providerops.Item,
	block IPBlockRecord, status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}
	transition := BlockOperationCompletion{Operation: completion, ItemStatus: status,
		BlockStatus: BlockStatusPendingReview}
	if status == providerops.StatusSucceeded {
		transition.RemoveBlock = true
	} else if status == providerops.StatusFailed {
		transition.BlockStatus, transition.ClearUnblock = BlockStatusActive, true
	}
	return w.repository.CompleteConnectionIPBlockOperation(ctx, block.ID, operation.Receipt.ID, item.Key,
		transition, w.now().UTC())
}
