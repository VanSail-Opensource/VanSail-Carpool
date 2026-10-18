package xnode

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type retryRepository struct{ cacheTestRepository }

func (*retryRepository) AcknowledgeXNodeState(context.Context, string, uint64, bool) error {
	return nil
}

func TestCommandAndSupersededFailuresCannotCreateResyncStorm(t *testing.T) {
	repo := &retryRepository{cacheTestRepository: cacheTestRepository{revision: 1}}
	service := NewService(repo, cacheTestSettings{}, nil, nil, nil)
	now := time.Now()
	service.now = func() time.Time { return now }
	master := NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}}
	command := make(chan wire.Message, 1)
	peer := &agentPeer{nodeID: "node", revision: 7, compiled: &compiledState{version: 1, config: config}, pending: map[string]chan wire.Message{"command": command}}
	prepared, err := preparePeerWire(peer, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	peer.digest = prepared.digest
	ctx := context.Background()
	for _, message := range []wire.Message{wire.New("error", "command", 0, map[string]any{}), wire.New("state.failed", "", 6, map[string]any{})} {
		if err = master.receive(ctx, peer, message); err != nil || peer.digest != prepared.digest {
			t.Fatal("unrelated failure invalidated current state", err)
		}
	}
	select {
	case <-command:
	default:
		t.Fatal("command waiter did not receive its error")
	}
	if err = master.receive(ctx, peer, wire.New("state.failed", "", 7, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	if peer.digest != "" || !peer.retryAt.After(now) {
		t.Fatal("current activation failure was ignored")
	}
	// No socket/AdvanceXNodeRevision implementation: any retry here would panic.
	for range 1000 {
		if err = master.syncNode(ctx, peer, true); err != nil {
			t.Fatal("unchanged failed target did not back off", err)
		}
	}
	if err = master.receive(ctx, peer, wire.New("state.applied", "", 7, map[string]any{})); err != nil {
		t.Fatal(err)
	}
	if peer.digest != prepared.digest || !peer.retryAt.IsZero() {
		t.Fatal("agent's autonomous recovery did not clear backoff")
	}
	if err = master.syncNode(ctx, peer, false); err != nil {
		t.Fatal("recovered state was resent", err)
	}
}
