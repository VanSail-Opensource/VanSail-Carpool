package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type renewalBudgetRepository struct {
	*renewalBeforeRoutingRepository
}

func (r *renewalBudgetRepository) XNodePAYGFunding(ctx context.Context, _ string) ([]PAYGFunding, error) {
	r.fundingReads.Add(1)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPartialFleetPassStillWakesDurableRoutingRecovery(t *testing.T) {
	service, base, _, master := compileCacheFixture(t)
	repo := &renewalBudgetRepository{&renewalBeforeRoutingRepository{
		paygLeaseRepository: &paygLeaseRepository{cacheTestRepository: base},
		nodes:               []Node{{ID: "slow", Enabled: true}},
	}}
	service.repo = repo
	peer := &agentPeer{nodeID: "slow", forceSync: true, compiled: &compiledState{version: 1,
		config: wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}}}}
	master.peers[peer.nodeID] = peer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := master.reconcile(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("fixture did not consume the fleet synchronization budget", err)
	}
	if repo.fundingReads.Load() != 1 || repo.routingCalls != 1 || master.routingWakePending.Load() {
		t.Fatalf("partial pass starved reconnect recovery: funding=%d wakes=%d pending=%t", repo.fundingReads.Load(), repo.routingCalls, master.routingWakePending.Load())
	}
	if ctx.Err() != nil || repo.observations != 0 || !peer.forceSync {
		t.Fatal("partial pass exceeded its deadline, billed observations or lost the node retry")
	}
}
