package xnode

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type aggregateMetricRepository struct{ profileMetricRepository }

func (aggregateMetricRepository) XNodePrincipalOwners(context.Context) (map[int]string, error) {
	return map[int]string{1: "member"}, nil
}

func (aggregateMetricRepository) XNodeNodes(context.Context) ([]Node, error) {
	return []Node{{ID: "node", Enabled: true}, {ID: "second", Enabled: true}}, nil
}
func (aggregateMetricRepository) XNodeNode(_ context.Context, id string) (Node, error) {
	return Node{ID: id, Enabled: true}, nil
}
func (aggregateMetricRepository) XNodeGroups(context.Context) ([]Group, error) {
	return []Group{{ID: "group", Enabled: true, Bindings: []GroupBinding{{NodeID: "node"}, {NodeID: "second"}}}}, nil
}

func aggregateMetricMaster(t *testing.T) *Master {
	t.Helper()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	service := NewService(aggregateMetricRepository{}, cacheTestSettings{}, nil, nil, nil)
	service.now = func() time.Time { return now }
	global := DefaultGlobal()
	global.ConnectionLimit = 2
	service.globalSettings.publish(global)
	master := NewMaster(service, nil)
	for _, node := range []string{"node", "second"} {
		master.peers[node] = &agentPeer{nodeID: node, openConnections: true}
		users := wire.UserMetrics{
			Connections:      map[int]int{1: math.MaxInt64},
			TotalConnections: map[int]int{1: math.MaxInt64},
			Rates:            map[int]wire.UserRate{
				1: {UploadBytesPerSecond: math.MaxFloat64, DownloadBytesPerSecond: 100, SampleSeconds: 1},
				2: {UploadBytesPerSecond: math.MaxFloat64, DownloadBytesPerSecond: 100, SampleSeconds: 1},
			},
		}
		if !validMetrics(users) {
			t.Fatal("fixture must pass authoritative per-node validation")
		}
		master.live[node] = liveSnapshot{receivedAt: now,
			hello:  wire.AgentHello{UpgradePending: true, Capabilities: []string{"sampled_activity_v1"}},
			report: wire.TelemetryReport{Ready: true, Synchronized: true, Time: now, Users: users}}
	}
	return master
}

func TestFleetAggregationKeepsCountsConservativeAndRatesFinite(t *testing.T) {
	master := aggregateMetricMaster(t)
	fleet, err := master.Metrics(context.Background())
	if err != nil || fleet.ActiveConnections != math.MaxInt64 || fleet.UploadBytesPerSecond != math.MaxFloat64 ||
		fleet.DownloadBytesPerSecond != 400 || fleet.TotalConnections == nil || *fleet.TotalConnections != "18446744073709551614" {
		t.Fatal("fleet aggregation overflowed or rounded exact totals", fleet, err)
	}
	for _, node := range fleet.Nodes {
		if node.ActiveConnections != math.MaxInt64 || node.UploadBytesPerSecond != math.MaxFloat64 {
			t.Fatal("per-node rate aggregation overflowed", node)
		}
	}
	member, err := master.UserMetrics(context.Background(), "member")
	if err != nil || member.Connections != math.MaxInt64 || member.UploadBytesPerSecond != math.MaxFloat64 ||
		member.DownloadBytesPerSecond != 200 || member.TotalConnections == nil || *member.TotalConnections != "18446744073709551614" {
		t.Fatal("member aggregation overflowed or lost exact totals", member, err)
	}
	for _, value := range []any{fleet, member} {
		if _, err := json.Marshal(value); err != nil {
			t.Fatal("finite input reports produced unencodable metrics", err)
		}
	}
}

func TestCrossNodeOverflowCannotReopenConnectionLimit(t *testing.T) {
	master := aggregateMetricMaster(t)
	if err := master.reconcile(context.Background()); err != nil || !master.Blocked(1) {
		t.Fatal("cross-node connection overflow disabled enforcement", err)
	}
	for node, snapshot := range master.live {
		snapshot.report.Users.Connections = map[int]int{1: 0}
		master.live[node] = snapshot
	}
	if err := master.reconcile(context.Background()); err != nil || master.Blocked(1) {
		t.Fatal("fresh cleared snapshots failed to release the limit", err)
	}
}
