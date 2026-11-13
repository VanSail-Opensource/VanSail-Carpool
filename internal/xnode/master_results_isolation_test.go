package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type blockedStateAcknowledgmentRepository struct {
	Repository
	entered chan struct{}
}

func (r *blockedStateAcknowledgmentRepository) AcknowledgeXNodeState(ctx context.Context, _ string, _ uint64, _ bool) error {
	close(r.entered)
	<-ctx.Done()
	return ctx.Err()
}

func TestSlowStateAcknowledgmentDoesNotDelayCommandResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := &blockedStateAcknowledgmentRepository{entered: make(chan struct{})}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	pending := make(chan wire.Message, 2)
	peer := &agentPeer{nodeID: "node", ctx: ctx, revision: 1, pending: map[string]chan wire.Message{"command": pending}}
	m.peers[peer.nodeID] = peer
	receivers := m.startPeerReceivers(peer)
	defer receivers.stop()
	if err := receivers.offer(wire.New("state.accepted", "state", 1, nil)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-repo.entered:
	case <-ctx.Done():
		t.Fatal("state acknowledgment did not reach the database")
	}
	for _, kind := range []string{"command.accepted", "command.result", "error"} {
		if err := receivers.offer(wire.New(kind, "command", 0, nil)); err != nil {
			t.Fatal(err)
		}
		select {
		case received := <-pending:
			if received.Type != kind {
				t.Fatal("receipt order changed", received.Type)
			}
		case <-ctx.Done():
			t.Fatal("state persistence blocked command receipt correlation")
		}
	}
}

type orderedStateAcknowledgmentRepository struct {
	Repository
	entered chan struct{}
	release chan struct{}
	drained chan struct{}
}

func (r *orderedStateAcknowledgmentRepository) AcknowledgeXNodeState(ctx context.Context, _ string, _ uint64, applied bool) error {
	if !applied {
		close(r.drained)
		return nil
	}
	close(r.entered)
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestStateErrorKeepsWireOrderWhileCommandResultsProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := &orderedStateAcknowledgmentRepository{entered: make(chan struct{}), release: make(chan struct{}), drained: make(chan struct{})}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	pending := make(chan wire.Message, 2)
	peer := &agentPeer{nodeID: "node", ctx: ctx, revision: 1, digest: "accepted-target", pending: map[string]chan wire.Message{"command": pending}}
	m.peers[peer.nodeID] = peer
	receivers := m.startPeerReceivers(peer)
	defer receivers.stop()
	if err := receivers.offer(wire.New("state.applied", "state", 1, nil)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-repo.entered:
	case <-ctx.Done():
		t.Fatal("state acknowledgment did not reach the database")
	}
	for _, frame := range []wire.Message{
		wire.New("error", "state", 1, nil),
		wire.New("command.result", "command", 0, nil),
		wire.New("state.accepted", "state", 1, nil),
	} {
		if err := receivers.offer(frame); err != nil {
			t.Fatal(err)
		}
	}
	// The receipt consumer must finish while the older state ACK is held. This
	// also forces a wrongly classified generic error to run before releasing it.
	select {
	case frame := <-pending:
		if frame.Type != "command.result" {
			t.Fatal(frame.Type)
		}
	case <-ctx.Done():
		t.Fatal("state persistence blocked an independent command result")
	}
	close(repo.release)
	select {
	case <-repo.drained:
	case <-ctx.Done():
		t.Fatal("state control frames did not drain")
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.digest != "" || peer.failedDigest != "accepted-target" || peer.retryAt.IsZero() {
		t.Fatal("an older applied ACK erased the later state rejection", peer.digest, peer.failedDigest, peer.retryAt)
	}
}
