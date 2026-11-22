package xnode

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

func syncFairnessFixture(count int) (*Master, *workersRepository, []*agentPeer) {
	repo := &workersRepository{started: make(chan string, count), release: make(chan struct{})}
	master := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	peers := make([]*agentPeer, count)
	for i := range peers {
		peers[i] = &agentPeer{nodeID: fmt.Sprintf("node-%02d", i)}
		master.peers[peers[i].nodeID] = peers[i]
	}
	return master, repo, peers
}

// Block both existing consumers, then end the pass while other peers are still
// queued. This exercises the actual worker dispatch without elapsed-time sleeps.
func cancelledSyncPair(t *testing.T, master *Master, repo *workersRepository, peers []*agentPeer) []string {
	t.Helper()
	// The fixture intentionally fails compilation. Advance the injected clock
	// between joined passes so the compile retry gate does not hide dispatch on
	// wraparound; no background master workers are running in this fixture.
	now := master.service.now().Add(2 * time.Second)
	master.service.now = func() time.Time { return now }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- master.syncPeers(ctx, peers, now) }()
	started := make([]string, 0, 2)
	for range 2 {
		select {
		case id := <-repo.started:
			started = append(started, id)
		case <-ctx.Done():
			t.Fatal("both fleet workers did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled pass returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled fleet workers did not join")
	}
	slices.Sort(started)
	if repo.active.Load() != 0 || repo.max.Load() > 2 {
		t.Fatalf("worker lifecycle/concurrency: active=%d max=%d", repo.active.Load(), repo.max.Load())
	}
	return started
}

func TestSyncWorkersResumeUnattemptedFleetBeforeRepeating(t *testing.T) {
	master, repo, peers := syncFairnessFixture(18)
	for pass := range 9 {
		got := cancelledSyncPair(t, master, repo, peers)
		want := []string{peers[2*pass].nodeID, peers[2*pass+1].nodeID}
		if !slices.Equal(got, want) {
			t.Fatalf("pass %d repeated an earlier peer before the fleet was visited: got %v want %v", pass, got, want)
		}
	}
	if got := cancelledSyncPair(t, master, repo, peers); !slices.Equal(got, []string{peers[0].nodeID, peers[1].nodeID}) {
		t.Fatalf("fleet cursor did not wrap: %v", got)
	}
}

func TestSyncWorkersPreCancelledPassDoesNotSkipPeers(t *testing.T) {
	master, repo, peers := syncFairnessFixture(4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := master.syncPeers(ctx, peers, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled pass returned %v", err)
	}
	select {
	case id := <-repo.started:
		t.Fatalf("pre-cancelled pass started peer %s", id)
	default:
	}
	if got := cancelledSyncPair(t, master, repo, peers); !slices.Equal(got, []string{peers[0].nodeID, peers[1].nodeID}) {
		t.Fatalf("pre-cancelled pass skipped unattempted peers: %v", got)
	}
}

func TestSyncWorkersOccupiedPeersCountAsAttempts(t *testing.T) {
	master, repo, peers := syncFairnessFixture(6)
	for _, peer := range peers[:2] {
		peer.syncMu.Lock()
		defer peer.syncMu.Unlock()
	}
	if got := cancelledSyncPair(t, master, repo, peers); !slices.Equal(got, []string{peers[2].nodeID, peers[3].nodeID}) {
		t.Fatalf("occupied peers blocked free workers: %v", got)
	}
	if got := cancelledSyncPair(t, master, repo, peers); !slices.Equal(got, []string{peers[4].nodeID, peers[5].nodeID}) {
		t.Fatalf("occupied peers prevented the cursor from advancing: %v", got)
	}
}
