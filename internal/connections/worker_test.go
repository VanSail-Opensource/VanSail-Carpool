package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestWorkerMovesAmbiguousStartsToPendingReview(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 18, 9, 30, 0, 0, time.UTC)
	remoteID := "50002"
	tests := []struct {
		name            string
		status          providerops.Status
		progress        float64
		requestErr      error
		wantRemoteCalls int
		wantUpdates     int
	}{
		{name: "interrupted processing", status: providerops.StatusProcessing, progress: 25, wantUpdates: 1},
		{name: "lost provider response", status: providerops.StatusQueued, requestErr: errors.New("response lost"), wantRemoteCalls: 1, wantUpdates: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &connectionWorkerRepository{
				scan: providerops.ConnectionScan{ID: "scan-1", UserID: "user-1", Status: test.status, ProgressPercent: test.progress},
				user: model.User{ID: "user-1", RemnaUserID: &remoteID},
			}
			remote := &connectionWorkerRemote{requestErr: test.requestErr}
			worker := NewWorker(repository, remote)
			worker.now = func() time.Time { return now }
			job := model.OutboxJob{Kind: ScanRequestOutboxKind, Payload: `{"scanId":"scan-1"}`}

			if err := worker.HandleOutbox(context.Background(), job); err != nil {
				t.Fatalf("HandleOutbox() error = %v", err)
			}
			if repository.scan.Status != providerops.StatusPendingReview ||
				repository.scan.ErrorCode != "CONNECTION_SCAN_START_AMBIGUOUS" || repository.scan.ProgressPercent != test.progress {
				t.Fatalf("scan = %+v", repository.scan)
			}
			if !projectMetadata(repository.scan).Failed {
				t.Fatal("pending-review projection did not terminate polling")
			}
			if remote.requestCalls != test.wantRemoteCalls || len(repository.updates) != test.wantUpdates {
				t.Fatalf("calls = %d, updates = %d", remote.requestCalls, len(repository.updates))
			}
			if err := worker.HandleOutbox(context.Background(), job); err != nil {
				t.Fatalf("replayed HandleOutbox() error = %v", err)
			}
			if remote.requestCalls != test.wantRemoteCalls || len(repository.updates) != test.wantUpdates {
				t.Fatalf("replay calls = %d, updates = %d", remote.requestCalls, len(repository.updates))
			}
		})
	}
}

type connectionWorkerRepository struct {
	scan    providerops.ConnectionScan
	user    model.User
	updates []providerops.ConnectionScanUpdate
}

func (r *connectionWorkerRepository) CreateConnectionScan(context.Context, providerops.ConnectionScanInput, time.Time) (providerops.ConnectionScan, bool, error) {
	return providerops.ConnectionScan{}, false, errors.New("unexpected CreateConnectionScan call")
}

func (r *connectionWorkerRepository) ConnectionScanForUser(context.Context, string, string) (providerops.ConnectionScan, error) {
	return providerops.ConnectionScan{}, errors.New("unexpected ConnectionScanForUser call")
}

func (r *connectionWorkerRepository) ConnectionScanByID(context.Context, string) (providerops.ConnectionScan, error) {
	return r.scan, nil
}

func (r *connectionWorkerRepository) UpdateConnectionScan(_ context.Context, _ string, update providerops.ConnectionScanUpdate, now time.Time) (providerops.ConnectionScan, error) {
	r.updates = append(r.updates, update)
	r.scan.Status = update.Status
	r.scan.ProgressPercent = update.ProgressPercent
	r.scan.ErrorCode = update.ErrorCode
	if update.ProviderJobID != "" {
		r.scan.ProviderJobID = update.ProviderJobID
	}
	if providerops.Terminal(update.Status) {
		r.scan.CompletedAt = &now
	}
	return r.scan, nil
}

func (r *connectionWorkerRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}

type connectionWorkerRemote struct {
	requestCalls int
	requestErr   error
}

func (r *connectionWorkerRemote) RequestConnectionScan(context.Context, string) (string, error) {
	r.requestCalls++
	return "", r.requestErr
}

func (r *connectionWorkerRemote) PollConnectionScan(context.Context, string) (ProviderScan, error) {
	return ProviderScan{}, errors.New("unexpected PollConnectionScan call")
}
