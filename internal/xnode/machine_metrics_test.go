package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type machineMetricsRepository struct {
	Repository // Any membership/accounting query is unexpected.
	nodes      []Node
	reads      int
	err        error
}

func (r *machineMetricsRepository) XNodeNodes(context.Context) ([]Node, error) {
	r.reads++
	return r.nodes, r.err
}

func TestMachineMetricsUseReceiptFreshnessWithoutMemberReads(t *testing.T) {
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name                string
		enabled, connected  bool
		age                 time.Duration
		pending             string
		stale, synchronized bool
	}{
		{"fresh", true, true, 10 * time.Second, "", false, true},
		{"expired", true, true, time.Minute, "", true, true},
		{"disconnected", true, false, 0, "", true, true},
		{"disabled", false, true, 0, "", true, true},
		{"firewall pending", true, true, 0, "receipt", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &machineMetricsRepository{nodes: []Node{{ID: "node", Enabled: test.enabled}}}
			service := NewService(repo, cacheTestSettings{}, nil, nil, nil)
			service.now = func() time.Time { return now }
			master := NewMaster(service, nil)
			if test.connected {
				master.peers["node"] = &agentPeer{}
			}
			raw := json.RawMessage(`{"cpu_usage":[1],"counter":18446744073709551615}`)
			received := now.Add(-test.age)
			master.live["node"] = liveSnapshot{receivedAt: received, firewallPending: test.pending,
				report: wire.TelemetryReport{Time: now.Add(time.Hour), Ready: true, Synchronized: true, Machine: raw}}
			got, err := master.MachineMetrics(context.Background())
			if err != nil || len(got) != 1 || repo.reads != 1 {
				t.Fatalf("machine-only read: %v, %+v, node reads=%d", err, got, repo.reads)
			}
			item := got[0]
			if item.Stale != test.stale || item.Synchronized != test.synchronized || !item.ReceivedAt.Equal(received) || !item.ValidUntil.Equal(received.Add(15*time.Second)) {
				t.Fatalf("receipt/readiness changed: %+v", item)
			}
			if test.stale {
				if string(item.Machine) != "{}" || item.ReportedAt != nil {
					t.Fatal("unavailable machine payload exposed", item)
				}
			} else {
				if string(item.Machine) != string(raw) || item.ReportedAt == nil {
					t.Fatal("machine data was rounded or browser-coerced", item)
				}
				item.Machine[0] = '['
				if master.live["node"].report.Machine[0] != '{' {
					t.Fatal("consumer mutated the live report bank")
				}
			}
		})
	}
}

func TestMachineMetricsDoNotInventReceiptOrHideStorageFailure(t *testing.T) {
	repo := &machineMetricsRepository{nodes: []Node{{ID: "node", Enabled: true}}}
	master := NewMaster(NewService(repo, cacheTestSettings{}, nil, nil, nil), nil)
	master.peers["node"] = &agentPeer{}
	got, err := master.MachineMetrics(context.Background())
	if err != nil || len(got) != 1 || !got[0].Stale || !got[0].ValidUntil.IsZero() {
		t.Fatal("missing report acquired freshness", got, err)
	}
	repo.err = errors.New("storage unavailable")
	if _, err := master.MachineMetrics(context.Background()); !errors.Is(err, repo.err) {
		t.Fatal("storage failure presented as an empty fleet", err)
	}
}

func TestCapturedMetricsKeepOneConsistentReportBank(t *testing.T) {
	master := NewMaster(NewService(nil, nil, nil, nil, nil), nil)
	peer := &agentPeer{openConnections: true}
	master.peers["node"] = peer
	master.live["node"] = liveSnapshot{report: wire.TelemetryReport{Users: wire.UserMetrics{Connections: map[int]int{7: 3}}}}
	captured := master.captureNodeObservations([]Node{{ID: "node"}})
	master.peers["node"] = &agentPeer{}
	master.live["node"] = liveSnapshot{report: wire.TelemetryReport{Users: wire.UserMetrics{Connections: map[int]int{}}}}
	if captured["node"].peer != peer || captured["node"].snapshot.report.Users.Connections[7] != 3 {
		t.Fatal("projection mixed old report data with a replacement session")
	}
}
