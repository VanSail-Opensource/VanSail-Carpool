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

type availabilityRepository struct{ Repository }

func (availabilityRepository) XNodeNodes(context.Context) ([]Node, error) {
	return []Node{{ID: "node", Enabled: true}}, nil
}
func (availabilityRepository) XNodeIdentities(context.Context) ([]Identity, error) {
	return []Identity{}, nil
}

func TestReadyLegacyAgentRemainsOnlineWithoutActivityCapability(t *testing.T) {
	service := NewService(availabilityRepository{}, cacheTestSettings{}, nil, nil, nil)
	master := NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	master.peers["node"] = &agentPeer{}
	master.live["node"] = liveSnapshot{receivedAt: time.Now(), report: wire.TelemetryReport{
		Ready: true, Synchronized: true, Machine: json.RawMessage(`{"cpu":{"usage":2}}`),
		Users: wire.UserMetrics{Connections: map[int]int{1: 99}},
	}}
	metrics, err := master.Metrics(context.Background())
	if err != nil || metrics.OnlineNodes != 1 || metrics.Nodes[0].Stale || metrics.ActiveConnections != 0 || len(metrics.Nodes[0].Machine) <= 2 {
		t.Fatal("ready legacy agent was hidden or unsampled counts admitted", err, metrics)
	}
	live := master.live["node"]
	live.receivedAt = time.Now().Add(-time.Minute)
	master.live["node"] = live
	metrics, err = master.Metrics(context.Background())
	if err != nil || metrics.OnlineNodes != 0 || !metrics.Nodes[0].Stale {
		t.Fatal("stale agent remained available", err)
	}
}

func TestFreshSnapshotsExpireAndMissingCountsAreNotMerged(t *testing.T) {
	now := time.Now()
	snapshot := liveSnapshot{receivedAt: now.Add(-14 * time.Second)}
	if !freshSnapshot(snapshot, now, 5) || freshSnapshot(snapshot, now.Add(2*time.Second), 5) {
		t.Fatal("freshness boundary is incorrect")
	}
	current := wire.UserMetrics{Connections: map[int]int{7: 2}, Status: map[int]wire.UserLimitStatus{}, Rates: map[int]wire.UserRate{}}
	if !validMetrics(current) {
		t.Fatal("valid snapshot rejected")
	}
	current.Connections[7] = -1
	if validMetrics(current) {
		t.Fatal("negative count accepted")
	}
	current.Connections = map[int]int{}
	if !validMetrics(current) {
		t.Fatal("authoritative empty snapshot rejected")
	}
}
