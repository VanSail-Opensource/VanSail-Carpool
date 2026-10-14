package connections

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// BlockWorkerRepository owns active block and operation lifecycle state.
type BlockWorkerRepository interface {
	UserByID(context.Context, string) (model.User, error)
	ConnectionIPBlockByOperation(context.Context, string) (IPBlockRecord, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteConnectionIPBlockOperation(context.Context, string, string, string, BlockOperationCompletion, time.Time) error
}

// BlockOperationCompletion atomically closes an operation and transitions its active block row.
type BlockOperationCompletion struct {
	Operation    providerops.Completion
	ItemStatus   providerops.Status
	BlockStatus  string
	RemoveBlock  bool
	ClearUnblock bool
}

// BlockRemote performs plugin execution and disconnect reconciliation via the queue.
type BlockRemote interface {
	BlockIP(context.Context, string, string, int) error
	UnblockIP(context.Context, string, string) error
	DropRemote
}

// BlockWorker blocks first, then disconnects and reconciles the selected IP.
type BlockWorker struct {
	repository BlockWorkerRepository
	remote     BlockRemote
	secrets    SecretBox
	now        func() time.Time
}

func NewBlockWorker(repository BlockWorkerRepository, remote BlockRemote, secrets SecretBox) *BlockWorker {
	return &BlockWorker{repository: repository, remote: remote, secrets: secrets, now: time.Now}
}

func (w *BlockWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	operation, item, block, ip, err := w.prepare(ctx, operation)
	if err != nil {
		return err
	}
	if item.Status == providerops.StatusProcessing {
		return w.finish(ctx, operation, item, block, providerops.StatusPendingReview, "BLOCK_OUTCOME_AMBIGUOUS")
	}
	if item.Status != providerops.StatusQueued {
		return nil
	}
	remaining := int(math.Ceil(block.ExpiresAt.Sub(w.now().UTC()).Seconds()))
	if remaining <= 0 {
		return w.finish(ctx, operation, item, block, providerops.StatusFailed, "BLOCK_WINDOW_EXPIRED")
	}
	item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	if err != nil {
		return err
	}
	if callErr := w.remote.BlockIP(ctx, ip, block.NodeUUID, remaining); callErr != nil {
		if w.remote.DefinitiveMutationFailure(callErr) {
			return w.finish(ctx, operation, item, block, providerops.StatusFailed, "BLOCK_REJECTED")
		}
		return w.finish(ctx, operation, item, block, providerops.StatusPendingReview, "BLOCK_OUTCOME_AMBIGUOUS")
	}
	return w.disconnect(ctx, operation, item, block, ip)
}

func (w *BlockWorker) prepare(ctx context.Context, operation providerops.Operation) (providerops.Operation, providerops.Item, IPBlockRecord, string, error) {
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", err
		}
	}
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "connection_ip_hmac" {
		return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", errors.New("connection block has an invalid target")
	}
	block, err := w.repository.ConnectionIPBlockByOperation(ctx, operation.Receipt.ID)
	if err != nil {
		return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", err
	}
	plaintext, err := w.secrets.Open(ipBlockSecretContext(block.UserID, block.NodeUUID, block.IPDigest), block.SealedIP)
	if err != nil || items[0].TargetID != block.IPDigest {
		return providerops.Operation{}, providerops.Item{}, IPBlockRecord{}, "", errors.New("connection block target could not be verified")
	}
	return operation, items[0], block, string(plaintext), nil
}
