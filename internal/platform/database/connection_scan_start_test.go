package database

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type scanStartBarrier struct {
	*Store
	arrived atomic.Int64
	ready   chan struct{}
	count   int64
}

func (r *scanStartBarrier) UserByID(ctx context.Context, id string) (model.User, error) {
	user, err := r.Store.UserByID(ctx, id)
	if err != nil {
		return user, err
	}
	if r.arrived.Add(1) == r.count {
		close(r.ready)
	}
	select {
	case <-ctx.Done():
		return model.User{}, ctx.Err()
	case <-r.ready:
		return user, nil
	}
}

type scanStartRemote struct {
	native atomic.Int64
	legacy atomic.Int64
}

func (r *scanStartRemote) RequestNativeConnectionScan(context.Context, string) (string, error) {
	r.native.Add(1)
	return "native-claimed-job", nil
}
func (r *scanStartRemote) RequestConnectionScan(context.Context, string) (string, error) {
	r.legacy.Add(1)
	return "", errors.New("unexpected legacy request")
}
func (*scanStartRemote) PollConnectionScan(context.Context, string) (connections.ProviderScan, error) {
	return connections.ProviderScan{}, errors.New("unexpected poll")
}

func TestConcurrentConnectionScanWorkersClaimOneExternalStart(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := newTestStore(t)
	owner := createTestUser(t, store, 900_602)
	service := connections.NewService(store, nil, nil)
	scan, err := service.RequestAdmin(ctx, scanAdministrator(t, store), owner, "concurrent-native-scan")
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	repository := &scanStartBarrier{Store: store, ready: make(chan struct{}), count: workers}
	remote := &scanStartRemote{}
	job := model.OutboxJob{Kind: connections.ScanRequestOutboxKind, Payload: `{"scanId":"` + scan.ID + `"}`}
	results := make(chan error, workers)
	var work sync.WaitGroup
	for range workers {
		work.Add(1)
		go func() {
			defer work.Done()
			results <- connections.NewWorker(repository, remote).HandleOutbox(ctx, job)
		}()
	}
	work.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrConflict) {
			conflicted++
		} else {
			t.Fatal("concurrent start returned unexpected error", err)
		}
	}
	if succeeded != 1 || conflicted != workers-1 || remote.native.Load() != 1 || remote.legacy.Load() != 0 {
		t.Fatal("queued snapshots duplicated external starts", succeeded, conflicted, remote.native.Load(), remote.legacy.Load())
	}
	current, err := store.ConnectionScanByID(ctx, scan.ID)
	if err != nil || current.Status != providerops.StatusProcessing || current.ProviderJobID != "native-claimed-job" || current.ErrorCode != "" {
		t.Fatal("winning job evidence lost", current, err)
	}
	if _, err := store.StartConnectionScan(ctx, scan.ID, time.Now().UTC()); !errors.Is(err, ErrConflict) {
		t.Fatal("existing start intent was admitted again", err)
	}
	if err := connections.NewWorker(store, remote).HandleOutbox(ctx, job); err != nil || remote.native.Load() != 1 {
		t.Fatal("retry duplicated retained provider job", err, remote.native.Load())
	}
	var jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='connection_scan_request' AND payload::jsonb->>'scanId'=$1`, scan.ID).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatal("scan start duplicated durable delivery", jobs, err)
	}
}
