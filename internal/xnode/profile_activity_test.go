package xnode

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestProfileIPsAreFreshOwnerScopedAndExact(t *testing.T) {
	m, now := profileMetricMaster(t, "native")
	m.peers["node"].activityReports = true
	snapshot := m.live["node"]
	snapshot.report.Users.SampledAt = &now
	snapshot.report.Users.ActiveIPObservations = []wire.IPActivity{
		{UserID: 1, IP: "8.8.8.8", LastActive: now, Observations: math.MaxUint64},
		{UserID: 1, IP: "::ffff:8.8.8.8", LastActive: now.Add(-time.Second), Observations: 1},
		{UserID: 2, IP: "9.9.9.9", LastActive: now, Observations: 1},
		{UserID: 1, IP: "10.0.0.1", LastActive: now, Observations: 1},
		{UserID: 1, IP: "1.1.1.1", LastActive: now.Add(-3 * time.Hour), Observations: 1},
	}
	m.live["node"] = snapshot
	items, err := m.UserNodeMetrics(context.Background(), "member")
	if err != nil || len(items) != 1 || items[0].ActiveIPStatus != "available" || len(items[0].ActiveIPs) != 2 {
		t.Fatal("missing or unscoped activity", items, err)
	}
	addresses := map[string]ProfileIPObservation{}
	for _, ip := range items[0].ActiveIPs {
		addresses[ip.IP] = ip
	}
	if value := addresses["8.8.8.8"]; !value.LastActive.Equal(now) || value.Observations != "18446744073709551616" || addresses["10.0.0.1"].Observations != "1" {
		t.Fatal("IP normalization, capture time or exact count was lost", value)
	}
	snapshot.receivedAt = now.Add(-time.Hour)
	m.live["node"] = snapshot
	items, err = m.UserNodeMetrics(context.Background(), "member")
	if err != nil || items[0].ActiveIPStatus != "stale" || len(items[0].ActiveIPs) != 0 || items[0].SampledAt == nil {
		t.Fatal("stale report claimed current activity", items, err)
	}
	delete(m.peers, "node")
	items, err = m.UserNodeMetrics(context.Background(), "member")
	if err != nil || items[0].ActiveIPStatus != "unavailable" {
		t.Fatal("disconnected node claimed available empty activity", items, err)
	}
}

func TestProfileIPRowsBoundedAndAvailableEmptyIsDistinct(t *testing.T) {
	now := time.Now().UTC()
	snapshot := liveSnapshot{report: wire.TelemetryReport{Users: wire.UserMetrics{SampledAt: &now}}}
	peer := &agentPeer{activityReports: true}
	item := UserNodeMetric{Available: true, ActiveIPStatus: "unavailable", ActiveIPs: []ProfileIPObservation{}}
	profileNodeActivity(&item, snapshot, peer, []int64{1}, now)
	if item.ActiveIPStatus != "available" || len(item.ActiveIPs) != 0 {
		t.Fatal("fresh empty observation became unavailable", item)
	}
	for i := range 300 {
		snapshot.report.Users.ActiveIPObservations = append(snapshot.report.Users.ActiveIPObservations, wire.IPActivity{UserID: 1, IP: fmt.Sprintf("8.8.%d.%d", i/256, i%256), LastActive: now, Observations: 1})
	}
	profileNodeActivity(&item, snapshot, peer, []int64{1}, now)
	if len(item.ActiveIPs) != maxProfileActiveIPs || !item.ActiveIPsTruncated {
		t.Fatal("IP projection exceeded its bound", len(item.ActiveIPs), item.ActiveIPsTruncated)
	}
	profileNodeActivity(&item, snapshot, peer, []int64{1}, now, 10)
	if len(item.ActiveIPs) != 10 || cap(item.ActiveIPs) > 16 || !item.ActiveIPsTruncated {
		t.Fatal("clipped projection retained the complete node history", len(item.ActiveIPs), cap(item.ActiveIPs))
	}
	profileNodeActivity(&item, snapshot, peer, []int64{1}, now, 0)
	if len(item.ActiveIPs) != 0 || !item.ActiveIPsTruncated || item.ActiveIPStatus != "available" {
		t.Fatal("response budget claimed a fresh empty report", item)
	}
}

type profileHistoryRepository struct{ profileMetricRepository }

func (r profileHistoryRepository) XNodePAYGNodeHistory(context.Context, string, time.Time, time.Time) ([]PAYGNodeUsageHour, bool, error) {
	return []PAYGNodeUsageHour{{NodeID: "removed", Name: "removed", UploadBytes: "9007199254740993", DownloadBytes: "3"}, {NodeID: "removed", Name: "removed", UploadBytes: "9007199254740993", DownloadBytes: "7"}}, false, nil
}

func TestProfileHistoryTotalsRemainExactAndImportedAccountingIsIndependent(t *testing.T) {
	s := NewService(profileHistoryRepository{}, nil, nil, nil, nil)
	history, err := s.profilePAYGHistory(context.Background(), "member")
	if err != nil || len(history.Nodes) != 1 || history.Nodes[0].UploadBytes != "18014398509481986" || history.Nodes[0].DownloadBytes != "10" {
		t.Fatal("node history rounded or omitted historical usage", history, err)
	}
	m, _ := profileMetricMaster(t, "native")
	metrics, err := m.ProfileMetrics(context.Background(), "member")
	if err != nil || metrics.Usage[6].NativeWeightedBytes != "9007199254741003" || metrics.Usage[6].WeightedBytes != "9007199254741048" || metrics.Nodes[0].NativeWeightedBytes != "9007199254741003" {
		t.Fatal("native presentation changed historical accounting", metrics, err)
	}
}
