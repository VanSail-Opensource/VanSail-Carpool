package connections

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type deniedScanRepository struct {
	connectionWorkerRepository
	clock      *time.Time
	admittedAt time.Time
	denialCode string
}

func (r *deniedScanRepository) UserByID(context.Context, string) (model.User, error) {
	*r.clock = r.clock.Add(time.Minute)
	return r.user, nil
}

func (r *deniedScanRepository) StartConnectionScan(_ context.Context, _ string, at time.Time) (providerops.ConnectionScan, error) {
	r.admittedAt = at
	r.scan.Status = providerops.StatusFailed
	r.scan.ErrorCode = r.denialCode
	return r.scan, nil
}

func TestScanAdmissionRefusalFencesAllProviderCalls(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"CONNECTION_SCAN_EXPIRED", "CONNECTION_SCAN_ACCOUNT_UNAVAILABLE"} {
		t.Run(code, func(t *testing.T) {
			now := time.Unix(100, 0)
			repo := &deniedScanRepository{clock: &now, denialCode: code, connectionWorkerRepository: connectionWorkerRepository{
				scan: providerops.ConnectionScan{ID: "scan", UserID: "member", Status: providerops.StatusQueued, RequestFingerprint: nativeScanFingerprint()},
				user: model.User{ID: "member"},
			}}
			remote := &nativeWorkerRemote{}
			worker := NewWorker(repo, remote)
			worker.now = func() time.Time { return now }
			job := model.OutboxJob{Kind: ScanRequestOutboxKind, Payload: `{"scanId":"scan"}`}
			if err := worker.HandleOutbox(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			if !repo.admittedAt.Equal(now) {
				t.Fatalf("admission used a clock from before the owner read: %v, want %v", repo.admittedAt, now)
			}
			if remote.nativeCalls != 0 || remote.requestCalls != 0 || len(repo.updates) != 0 || repo.scan.ErrorCode != code {
				t.Fatal("admission refusal was bypassed or overwritten", repo.scan, remote)
			}
			if err := worker.HandleOutbox(context.Background(), job); err != nil || remote.nativeCalls != 0 {
				t.Fatal("terminal replay invoked provider", err)
			}
		})
	}
}

type terminalUnblockRepository struct{ *blockWorkerRepo }

func (r *terminalUnblockRepository) BeginProviderOperationItemAttempt(_ context.Context, _, _ string, _ time.Time) (providerops.Item, error) {
	r.item.Status = providerops.StatusFailed
	return r.item, nil
}

func TestUnblockClaimPreservesConcurrentTerminalResult(t *testing.T) {
	t.Parallel()
	repo, remote, _ := newBlockWorkerFixture(time.Now().UTC())
	repo.operation.Receipt.Kind = UnblockOperationKind
	worker := NewUnblockWorker(&terminalUnblockRepository{repo}, remote, workerSecrets{})
	if err := worker.HandleProviderOperation(context.Background(), repo.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if len(remote.events) != 0 || repo.deleted || repo.item.Status != providerops.StatusFailed {
		t.Fatal("concurrent terminal claim was bypassed", repo.item, remote.events)
	}
}
