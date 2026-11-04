package connections

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type blockWorkerRepo struct {
	operation providerops.Operation
	item      providerops.Item
	block     IPBlockRecord
	user      model.User
	deleted   bool
}

func (r *blockWorkerRepo) UserByID(context.Context, string) (model.User, error) { return r.user, nil }
func (r *blockWorkerRepo) ConnectionIPBlockByOperation(context.Context, string) (IPBlockRecord, error) {
	return r.block, nil
}
func (r *blockWorkerRepo) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{r.item}, nil
}
func (r *blockWorkerRepo) BeginProviderOperationAttempt(_ context.Context, _ string, at time.Time) (providerops.Operation, error) {
	r.operation.Receipt.Status, r.operation.Attempts, r.operation.AttemptStartedAt = string(providerops.StatusProcessing), 1, &at
	return r.operation, nil
}
func (r *blockWorkerRepo) BeginProviderOperationItemAttempt(_ context.Context, _, _ string, at time.Time) (providerops.Item, error) {
	r.item.Status, r.item.AttemptStartedAt = providerops.StatusProcessing, &at
	return r.item, nil
}
func (r *blockWorkerRepo) CompleteConnectionIPBlockOperation(_ context.Context, _, _, _ string,
	completion BlockOperationCompletion, at time.Time) error {
	r.item.Status, r.item.ErrorCode, r.item.CompletedAt = completion.ItemStatus, completion.Operation.ErrorCode, &at
	r.operation.Receipt.Status, r.operation.Receipt.ErrorCode, r.operation.Receipt.CompletedAt = string(completion.Operation.Status), nil, &at
	if completion.Operation.ErrorCode != "" {
		r.operation.Receipt.ErrorCode = &completion.Operation.ErrorCode
	}
	if completion.RemoveBlock {
		r.deleted = true
	} else {
		r.block.Status = completion.BlockStatus
		if completion.ClearUnblock {
			r.block.UnblockOperationID = ""
		}
	}
	return nil
}

type workerSecrets struct{}

func (workerSecrets) Seal(string, []byte) (string, error) { return "cipher", nil }
func (workerSecrets) Open(string, string) ([]byte, error) { return []byte("203.0.113.8"), nil }

type blockWorkerRemote struct {
	events                                 []string
	blockErr, dropErr, scanErr, unblockErr error
	definitive                             bool
}

func (r *blockWorkerRemote) BlockIP(_ context.Context, _ string, _ string, timeout int) error {
	r.events = append(r.events, "block")
	if timeout != 259200 {
		return errors.New("wrong timeout")
	}
	return r.blockErr
}
func (r *blockWorkerRemote) UnblockIP(context.Context, string, string) error {
	r.events = append(r.events, "unblock")
	return r.unblockErr
}
func (r *blockWorkerRemote) DropConnection(context.Context, string, string) error {
	r.events = append(r.events, "drop")
	return r.dropErr
}
func (r *blockWorkerRemote) RequestConnectionScan(context.Context, string) (string, error) {
	return "scan", r.scanErr
}
func (r *blockWorkerRemote) PollConnectionScan(context.Context, string) (ProviderScan, error) {
	return ProviderScan{Completed: true}, nil
}
func (r *blockWorkerRemote) DefinitiveMutationFailure(error) bool { return r.definitive }

func newBlockWorkerFixture(now time.Time) (*blockWorkerRepo, *blockWorkerRemote, *UnblockWorker) {
	remoteID := "42"
	repo := &blockWorkerRepo{
		operation: providerops.Operation{Receipt: model.OperationReceipt{ID: "operation", Kind: BlockOperationKind, Status: "queued"}, OwnerUserID: "owner"},
		item:      providerops.Item{OperationID: "operation", Key: "ip", TargetType: "connection_ip_hmac", TargetID: "digest", Status: providerops.StatusQueued},
		block:     IPBlockRecord{ID: "block", UserID: "owner", NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee", IPDigest: "digest", SealedIP: "cipher", Status: BlockStatusBlocking, ExpiresAt: now.Add(72 * time.Hour)},
		user:      model.User{ID: "owner", ProxyUserID: &remoteID},
	}
	remote := &blockWorkerRemote{}
	worker := NewUnblockWorker(repo, remote, workerSecrets{})
	worker.now = func() time.Time { return now }
	return repo, remote, worker
}
