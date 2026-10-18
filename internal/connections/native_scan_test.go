package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type nativeWorkerRemote struct {
	connectionWorkerRemote
	nativeCalls int
	target      string
}

func (r *nativeWorkerRemote) RequestNativeConnectionScan(_ context.Context, id string) (string, error) {
	r.nativeCalls++
	r.target = id
	return "xna:42:nonce", nil
}

func TestNativeAdminScanUsesPanelIdentityAndNeverStartsTheLegacyReader(t *testing.T) {
	repo := &connectionWorkerRepository{scan: providerops.ConnectionScan{ID: "native", UserID: "member", Status: providerops.StatusQueued, RequestFingerprint: nativeScanFingerprint()}, user: model.User{ID: "member"}}
	remote := &nativeWorkerRemote{}
	worker := NewWorker(repo, remote)
	job := model.OutboxJob{Kind: ScanRequestOutboxKind, Payload: `{"scanId":"native"}`}
	if err := worker.HandleOutbox(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if remote.nativeCalls != 1 || remote.target != "member" || remote.requestCalls != 0 || repo.scan.ProviderJobID != "xna:42:nonce" {
		t.Fatal("native scan escaped its identity/executor boundary", repo.scan, remote)
	}
	if err := worker.HandleOutbox(context.Background(), job); err != nil || remote.nativeCalls != 1 {
		t.Fatal("retry duplicated a native start", err)
	}
}

func TestAdminPollRefusesALegacyScanBeforeCallingAnyReader(t *testing.T) {
	repo := &targetScopedScanRepository{scan: providerops.ConnectionScan{ID: "legacy", UserID: "member", Status: providerops.StatusProcessing, ProviderJobID: "legacy-job", RequestFingerprint: scanFingerprint(), ExpiresAt: time.Now().Add(time.Minute)}}
	service := NewService(repo, nil, nil)
	if _, err := service.PollAdmin(context.Background(), "member", "legacy"); !errors.Is(err, ErrScanNotFound) {
		t.Fatal("administrator polled a legacy job", err)
	}
}
