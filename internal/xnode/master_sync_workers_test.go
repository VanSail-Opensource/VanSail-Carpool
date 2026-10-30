package xnode

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type workersRepository struct {
	Repository
	active, max atomic.Int32
	started     chan string
	release     chan struct{}
}

func (r *workersRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return 1, time.Time{}, nil
}
func (r *workersRepository) XNodeNode(ctx context.Context, id string) (Node, error) {
	active := r.active.Add(1)
	defer r.active.Add(-1)
	for previous := r.max.Load(); active > previous; previous = r.max.Load() {
		if r.max.CompareAndSwap(previous, active) {
			break
		}
	}
	r.started <- id
	select {
	case <-r.release:
	case <-ctx.Done():
	}
	return Node{}, ErrUnavailable
}
func TestSyncWorkersBoundConcurrencyAndSkipOccupiedPeer(t *testing.T) {
	r := &workersRepository{started: make(chan string, 4), release: make(chan struct{})}
	m := NewMaster(NewService(r, nil, nil, nil, nil), nil)
	peers := []*agentPeer{{nodeID: "held"}, {nodeID: "a"}, {nodeID: "b"}, {nodeID: "c"}}
	peers[0].syncMu.Lock()
	defer peers[0].syncMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- m.syncPeers(ctx, peers, time.Now()) }()
	for range 2 {
		select {
		case <-r.started:
		case <-ctx.Done():
			t.Fatal("occupied peer blocked fleet")
		}
	}
	if r.max.Load() != 2 {
		t.Fatalf("worker concurrency=%d", r.max.Load())
	}
	close(r.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if r.max.Load() > 2 || r.active.Load() != 0 {
		t.Fatal("workers exceeded bound or did not join")
	}
}
