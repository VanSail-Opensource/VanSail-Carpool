package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type logRepository struct{ availabilityRepository }

func (logRepository) XNodeNode(_ context.Context, id string) (Node, error) {
	if id == "node" {
		return Node{ID: id, Enabled: true}, nil
	}
	return Node{}, ErrNotFound
}

func TestSummaryOmitsLogsAndSelectedNodeRetainsRealtimeBuffer(t *testing.T) {
	s := NewService(logRepository{}, cacheTestSettings{}, nil, nil, nil)
	m := NewMaster(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	peer := &agentPeer{nodeID: "node"}
	m.peers["node"] = peer
	m.live["node"] = liveSnapshot{receivedAt: time.Now(), report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	for i := 0; i < 2; i++ {
		entries := make([]json.RawMessage, 300)
		for j := range entries {
			entries[j] = json.RawMessage(`{"time":"2026-10-20T00:00:00Z","source":"xray","text":"live entry"}`)
		}
		if err := m.receive(context.Background(), peer, wire.New("logs.report", "", 0, map[string]any{"entries": entries, "dropped": uint64(9007199254740993)})); err != nil {
			t.Fatal(err)
		}
	}
	full, err := m.Metrics(context.Background())
	if err != nil || len(full.Nodes[0].Logs) != 500 {
		t.Fatal(err)
	}
	light, err := m.Metrics(context.Background(), true)
	if err != nil || len(light.Nodes[0].Logs) != 0 {
		t.Fatal("summary copied log payload", err)
	}
	logs, err := m.Logs(context.Background(), "node")
	if err != nil || len(logs.Entries) != 500 || logs.Stale || !logs.Connected || logs.Dropped != "18014398509481986" {
		t.Fatal(logs, err)
	}
	logs.Entries[0] = nil
	if m.live["node"].logs[0] == nil {
		t.Fatal("response aliases mutable buffer headers")
	}
	delete(m.peers, "node")
	offline, err := m.Logs(context.Background(), "node")
	if err != nil || !offline.Stale || offline.Connected || len(offline.Entries) != 500 {
		t.Fatal("offline state lost buffered logs", err)
	}
	if _, err := m.Logs(context.Background(), "unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
