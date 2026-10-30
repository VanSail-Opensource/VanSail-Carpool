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

type connectedMemberRepository struct{ availabilityRepository }

func (connectedMemberRepository) XNodePrincipalOwners(context.Context) (map[int]string, error) {
	return map[int]string{1: "member", 2: "member", 3: "other"}, nil
}

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

func TestPAYGReadinessUsesConfiguredReportInterval(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, nil, nil, nil)
	service.now = func() time.Time { return now }
	master := NewMaster(service, nil)
	master.peers["node"] = &agentPeer{paygReady: true}
	master.live["node"] = liveSnapshot{receivedAt: now.Add(-30 * time.Second), report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	if !master.paygNodeReady("node", 20) || master.paygNodeReady("node", 5) {
		t.Fatal("PAYG readiness did not use the configured telemetry window")
	}
	master.peers["node"].paygReady = false
	if master.paygNodeReady("node", 20) {
		t.Fatal("unrecovered PAYG node became ready")
	}
}

func TestFleetOpenConnectionsRequireFreshNegotiatedCoverage(t *testing.T) {
	for _, test := range []struct {
		name              string
		negotiated, stale bool
		want              string
	}{
		{"open includes idle", true, false, "9223372036854775814"},
		{"legacy", false, false, ""},
		{"stale", true, true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(availabilityRepository{}, cacheTestSettings{}, nil, nil, nil)
			master := NewMaster(service, nil)
			master.peers["node"] = &agentPeer{openConnections: test.negotiated}
			at := time.Now()
			if test.stale {
				at = at.Add(-time.Minute)
			}
			master.live["node"] = liveSnapshot{receivedAt: at,
				hello: wire.AgentHello{Capabilities: []string{"sampled_activity_v1"}},
				report: wire.TelemetryReport{Ready: true, Synchronized: true,
					Users: wire.UserMetrics{Connections: map[int]int{1: 2}, TotalConnections: map[int]int{1: 9223372036854775807, 2: 7}}}}
			got, err := master.Metrics(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if got.TotalConnections != nil || got.Nodes[0].TotalConnections != nil {
					t.Fatal("fabricated total", got)
				}
			} else if got.TotalConnections == nil || *got.TotalConnections != test.want || got.Nodes[0].TotalConnections == nil || *got.Nodes[0].TotalConnections != test.want || got.ActiveConnections != 2 {
				t.Fatal("open and sampled counters were conflated", got)
			}
		})
	}
}

func TestNodeOnlineMembersDeduplicatePaidAndPAYGOpenFlows(t *testing.T) {
	service := NewService(connectedMemberRepository{}, cacheTestSettings{}, nil, nil, nil)
	master := NewMaster(service, nil)
	master.peers["node"] = &agentPeer{openConnections: true}
	master.live["node"] = liveSnapshot{receivedAt: time.Now(),
		hello: wire.AgentHello{Capabilities: []string{"sampled_activity_v1"}},
		report: wire.TelemetryReport{Ready: true, Synchronized: true,
			Users: wire.UserMetrics{Connections: map[int]int{3: 1}, TotalConnections: map[int]int{1: 2, 2: 3, 3: 0, 99: 7}}}}
	got, err := master.Metrics(context.Background())
	if err != nil || got.Nodes[0].UsersOnline != 1 || !got.Nodes[0].UsersOnlineAvailable {
		t.Fatal("idle members were missed or paid/PAYG/service identities double-counted", got, err)
	}
	master.peers["node"].openConnections = false
	got, err = master.Metrics(context.Background())
	if err != nil || got.Nodes[0].UsersOnlineAvailable {
		t.Fatal("legacy count claimed complete", got, err)
	}
}
