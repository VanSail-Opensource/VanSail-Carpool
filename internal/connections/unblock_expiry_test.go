package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestManualUnblockDeletesActiveRowAfterAcceptance(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	repo, remote, _ := newBlockWorkerFixture(now)
	repo.operation.Receipt.Kind = UnblockOperationKind
	repo.block.ExpiresAt = now.Add(time.Hour)
	worker := NewUnblockWorker(repo, remote, workerSecrets{})
	worker.now = func() time.Time { return now }

	if err := worker.HandleProviderOperation(context.Background(), repo.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if !repo.deleted || repo.operation.Receipt.Status != "succeeded" || len(remote.events) != 1 || remote.events[0] != "unblock" {
		t.Fatalf("repo=%+v events=%v", repo, remote.events)
	}
}

func TestDefinitiveUnblockFailureAllowsRetry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	repo, remote, _ := newBlockWorkerFixture(now)
	repo.operation.Receipt.Kind, repo.block.UnblockOperationID = UnblockOperationKind, repo.operation.Receipt.ID
	remote.unblockErr, remote.definitive = errors.New("rejected"), true
	worker := NewUnblockWorker(repo, remote, workerSecrets{})
	worker.now = func() time.Time { return now }

	if err := worker.HandleProviderOperation(context.Background(), repo.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if repo.operation.Receipt.Status != "failed" || repo.block.Status != BlockStatusActive || repo.block.UnblockOperationID != "" {
		t.Fatalf("repo = %+v", repo)
	}
}

type expiryRepo struct {
	block   IPBlockRecord
	deleted bool
}

func (r *expiryRepo) ConnectionIPBlockByID(context.Context, string) (IPBlockRecord, error) {
	return r.block, nil
}
func (r *expiryRepo) FinalizeConnectionIPBlockExpiry(context.Context, string, bool, time.Time) error {
	r.deleted = true
	return nil
}

func TestExpiryRetriesAndScrubsAfterExhaustion(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	repo := &expiryRepo{block: IPBlockRecord{ID: "block", UserID: "owner", NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee",
		IPDigest: "digest", SealedIP: "cipher", ExpiresAt: now}}
	remote := &blockWorkerRemote{unblockErr: errors.New("provider unavailable")}
	worker := NewExpiryWorker(repo, remote, workerSecrets{})
	worker.now = func() time.Time { return now }
	job := model.OutboxJob{Kind: BlockExpiryOutboxKind, Payload: `{"blockId":"block"}`, Attempts: 9}
	if err := worker.HandleOutbox(context.Background(), job); err == nil || repo.deleted {
		t.Fatalf("attempt 9 err=%v deleted=%t", err, repo.deleted)
	}
	job.Attempts = 10
	if err := worker.HandleOutbox(context.Background(), job); err == nil || !repo.deleted {
		t.Fatalf("attempt 10 err=%v deleted=%t", err, repo.deleted)
	}
}

var _ BlockWorkerRepository = (*blockWorkerRepo)(nil)
var _ providerops.Handler = (*UnblockWorker)(nil)
