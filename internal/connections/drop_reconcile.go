package connections

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const dropReconcileTimeout = 5 * time.Second

func (w *DropWorker) pollReconciliation(ctx context.Context, jobID string) (ProviderScan, bool) {
	ctx, cancel := context.WithTimeout(ctx, dropReconcileTimeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := w.remote.PollConnectionScan(ctx, jobID)
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

func connectionPresent(scan ProviderScan, nodeUUID, ip string) bool {
	for _, node := range scan.Nodes {
		if node.UUID != nodeUUID {
			continue
		}
		for _, observation := range node.IPs {
			if observation.Address == ip {
				return true
			}
		}
	}
	return false
}

func (w *DropWorker) finish(ctx context.Context, operation providerops.Operation, item providerops.Item, status providerops.Status, code string) error {
	completion := providerops.Completion{Status: status, ErrorCode: code, ResultJSON: "{}"}
	if item.Status == providerops.StatusQueued {
		var err error
		item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
		if err != nil {
			return err
		}
	}
	if item.Status == providerops.StatusProcessing {
		if _, err := w.repository.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key, completion, w.now().UTC()); err != nil {
			return err
		}
	}
	_, err := w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, w.now().UTC())
	return err
}
