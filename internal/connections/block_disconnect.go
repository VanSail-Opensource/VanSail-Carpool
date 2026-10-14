package connections

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *BlockWorker) disconnect(ctx context.Context, operation providerops.Operation, item providerops.Item,
	block IPBlockRecord, ip string) error {
	user, err := w.repository.UserByID(ctx, operation.OwnerUserID)
	if err != nil || user.RemnaUserID == nil {
		return w.finish(ctx, operation, item, block, providerops.StatusPartial, "DROP_RECONCILIATION_FAILED")
	}
	callErr := w.remote.DropConnection(ctx, ip, block.NodeUUID)
	if callErr == nil {
		return w.finish(ctx, operation, item, block, providerops.StatusSucceeded, "")
	}
	jobID, err := w.remote.RequestConnectionScan(ctx, *user.RemnaUserID)
	if err != nil {
		return w.finish(ctx, operation, item, block, providerops.StatusPartial, "DROP_RECONCILIATION_FAILED")
	}
	result, completed := pollBlockReconciliation(ctx, w.remote, jobID)
	if completed && !connectionPresent(result, block.NodeUUID, ip) {
		return w.finish(ctx, operation, item, block, providerops.StatusSucceeded, "")
	}
	if w.remote.DefinitiveMutationFailure(callErr) {
		return w.finish(ctx, operation, item, block, providerops.StatusPartial, "DROP_REJECTED")
	}
	return w.finish(ctx, operation, item, block, providerops.StatusPartial, "DROP_OUTCOME_AMBIGUOUS")
}

func pollBlockReconciliation(ctx context.Context, remote BlockRemote, jobID string) (ProviderScan, bool) {
	ctx, cancel := context.WithTimeout(ctx, dropReconcileTimeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := remote.PollConnectionScan(ctx, jobID)
		if err == nil && (result.Completed || result.Failed) {
			return result, result.Completed && !result.Failed
		}
		select {
		case <-ctx.Done():
			return ProviderScan{}, false
		case <-ticker.C:
		}
	}
}

func (w *BlockWorker) finish(ctx context.Context, operation providerops.Operation, item providerops.Item,
	block IPBlockRecord, status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}
	itemStatus := status
	blockStatus := BlockStatusActive
	remove := status == providerops.StatusFailed
	if status == providerops.StatusPartial {
		itemStatus = providerops.StatusSucceeded
	} else if status == providerops.StatusPendingReview {
		blockStatus = BlockStatusPendingReview
	}
	return w.repository.CompleteConnectionIPBlockOperation(ctx, block.ID, operation.Receipt.ID, item.Key,
		BlockOperationCompletion{Operation: completion, ItemStatus: itemStatus, BlockStatus: blockStatus,
			RemoveBlock: remove}, w.now().UTC())
}
