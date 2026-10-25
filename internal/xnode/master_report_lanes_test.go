package xnode

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type blockingReportRepository struct {
	Repository
	entered chan struct{}
	calls   atomic.Int32
}

func (r *blockingReportRepository) RecordXNodeTraffic(ctx context.Context, _, _ string, _ uint64, _ string, _ wire.TrafficReport) error {
	r.calls.Add(1)
	select {
	case r.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestReportLanesKeepTelemetryAndCommandsFreshDuringBlockedAccounting(t *testing.T) {
	repo := &blockingReportRepository{entered: make(chan struct{}, 1)}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pending := make(chan wire.Message, 1)
	peer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "boot", ctx: ctx, pending: map[string]chan wire.Message{"command": pending}}
	m.peers[peer.nodeID] = peer
	r := m.startPeerReceivers(peer)
	defer r.stop()
	if err := r.offer(wire.New("traffic.report", "agent:1", 0, wire.TrafficReport{ID: "agent:1"})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-repo.entered:
	case <-time.After(time.Second):
		t.Fatal("accounting did not begin")
	}
	if err := r.offer(wire.New("traffic.report", "agent:2", 0, wire.TrafficReport{ID: "agent:2"})); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		r.offer(wire.New("logs.report", "", 0, map[string]any{"entries": []any{}, "dropped": 0}))
		r.offer(wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: "agent", BootID: "boot", Ready: true, ConfigRevision: 2}))
	}
	r.offer(wire.New("command.result", "command", 0, nil))
	select {
	case <-pending:
	case <-time.After(time.Second):
		t.Fatal("accounting starved command results")
	}
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.RLock()
		fresh := m.live[peer.nodeID].report.ConfigRevision == 2
		m.mu.RUnlock()
		if fresh {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("telemetry missed two 500ms reporting intervals")
		}
		time.Sleep(time.Millisecond)
	}
	if repo.calls.Load() != 1 {
		t.Fatal("traffic batches processed concurrently", repo.calls.Load())
	}
}

func TestInboundTrafficLaneEnforcesByteAndFrameLimits(t *testing.T) {
	lane := newInboundLane(1)
	if !lane.offer(wire.Message{}) || lane.offer(wire.Message{}) {
		t.Fatal("frame bound not enforced")
	}
	lane = newInboundLane(8)
	if lane.offer(wire.Message{Data: make([]byte, 32<<20)}) || lane.bytes.Load() != 0 {
		t.Fatal("byte bound not released after rejection")
	}
}
