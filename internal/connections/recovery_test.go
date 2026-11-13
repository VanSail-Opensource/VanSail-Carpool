package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type connectionRecoveryRepository struct {
	connectionWorkerRepository
	lookupErr error
	ownerErr  error
}

func (r *connectionRecoveryRepository) ConnectionScanForUser(context.Context, string, string) (providerops.ConnectionScan, error) {
	return r.scan, r.lookupErr
}
func (r *connectionRecoveryRepository) UserByID(_ context.Context, id string) (model.User, error) {
	if id == scanTestAdministrator().ID {
		return scanTestAdministrator(), nil
	}
	return r.user, r.ownerErr
}

func TestConnectionPollPreservesStorageFailureForRetry(t *testing.T) {
	t.Parallel()
	cause := errors.New("temporary database outage")
	repo := &connectionRecoveryRepository{lookupErr: cause}
	service := NewService(repo, nil, nil)
	if _, err := service.PollAdmin(context.Background(), scanTestAdministrator(), "member", "scan"); !errors.Is(err, cause) || !errors.Is(err, ErrScanUnavailable) || errors.Is(err, ErrScanNotFound) {
		t.Fatal("storage failure misclassified as missing scan", err)
	}
	repo.lookupErr = ErrScanNotFound
	if _, err := service.PollAdmin(context.Background(), scanTestAdministrator(), "member", "scan"); !errors.Is(err, ErrScanNotFound) || errors.Is(err, ErrScanUnavailable) {
		t.Fatal("missing scan lost its privacy boundary", err)
	}
	repo.lookupErr = nil
	repo.scan = providerops.ConnectionScan{ID: "scan", UserID: "member", Status: providerops.StatusQueued,
		RequestFingerprint: nativeScanFingerprint(), ExpiresAt: time.Now().Add(time.Minute)}
	if scan, err := service.PollAdmin(context.Background(), scanTestAdministrator(), "member", "scan"); err != nil || scan.ID != "scan" || scan.Failed {
		t.Fatal("scan failed to recover after storage became available", scan, err)
	}
}

func TestConnectionOwnerReadFailureLeavesStartUnattempted(t *testing.T) {
	t.Parallel()
	cause := errors.New("owner read unavailable")
	repo := &connectionRecoveryRepository{ownerErr: cause, connectionWorkerRepository: connectionWorkerRepository{
		scan: providerops.ConnectionScan{ID: "scan", UserID: "member", Status: providerops.StatusQueued, RequestFingerprint: nativeScanFingerprint()},
		user: model.User{ID: "member"},
	}}
	remote := &nativeWorkerRemote{}
	worker := NewWorker(repo, remote)
	job := model.OutboxJob{Kind: ScanRequestOutboxKind, Payload: `{"scanId":"scan"}`}
	if err := worker.HandleOutbox(context.Background(), job); !errors.Is(err, cause) {
		t.Fatal("owner read failure was hidden", err)
	}
	if repo.scan.Status != providerops.StatusQueued || len(repo.updates) != 0 || remote.nativeCalls != 0 || remote.requestCalls != 0 {
		t.Fatal("local read consumed the external start", repo.scan, remote)
	}
	repo.ownerErr = nil
	if err := worker.HandleOutbox(context.Background(), job); err != nil || remote.nativeCalls != 1 || repo.scan.ProviderJobID != "xna:42:nonce" {
		t.Fatal("unattempted scan did not recover", repo.scan, remote, err)
	}
	if err := worker.HandleOutbox(context.Background(), job); err != nil || remote.nativeCalls != 1 {
		t.Fatal("completed start replayed the provider request", err, remote)
	}
}
