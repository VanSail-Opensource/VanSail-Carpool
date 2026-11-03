package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestActivityTimestampCannotBeRefreshedByReportingOldSamples(t *testing.T) {
	now := time.Now().UTC()
	sampled := now.Add(-time.Minute)
	snapshot := liveSnapshot{activityReports: true, receivedAt: now, hello: wire.AgentHello{Capabilities: []string{"sampled_activity_v1"}}, report: wire.TelemetryReport{Ready: true, Synchronized: true, Users: wire.UserMetrics{SampledAt: &sampled}}}
	if activityAvailable(snapshot, now, 2, 1) {
		t.Fatal("new report refreshed an old sample")
	}
	if !activityAvailable(snapshot, now, 2, 60) {
		t.Fatal("valid slow sampling interval ignored")
	}
	snapshot.report.Users.SampledAt = nil
	if activityAvailable(snapshot, now, 2, 1) {
		t.Fatal("unsampled negotiated agent presented as idle")
	}
	snapshot.activityReports = false
	if !activityAvailable(snapshot, now, 2, 1) {
		t.Fatal("legacy measured-agent telemetry decoding changed")
	}
}

func TestActivityFramesRequireNegotiationAndCannotReplaceNewerSamples(t *testing.T) {
	for _, compact := range []bool{false, true} {
		service := NewService(nil, nil, nil, nil, nil)
		m := NewMaster(service, nil)
		peer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "boot", compactTelemetry: compact}
		m.peers["node"] = peer
		at := time.Now().UTC()
		report := wire.TelemetryReport{AgentID: "agent", BootID: "boot", Time: at, Users: wire.UserMetrics{SampledAt: &at, ActiveIPObservations: []wire.IPActivity{{UserID: 7, IP: "8.8.8.8", LastActive: at, Observations: 1}}}}
		frame := func() wire.Message {
			if compact {
				return wire.New("telemetry.compact", "", 0, wire.CompactTelemetry(report))
			}
			return wire.New("telemetry.report", "", 0, report)
		}
		if err := m.receive(context.Background(), peer, frame()); err == nil {
			t.Fatal("unnegotiated extension accepted", compact)
		}
		peer.activityReports = true
		if err := m.receive(context.Background(), peer, frame()); err != nil {
			t.Fatal("negotiated activity rejected", compact, err)
		}
		older := at.Add(-time.Second)
		report.Users.SampledAt = &older
		report.Users.ActiveIPObservations[0].LastActive = older
		if err := m.receive(context.Background(), peer, frame()); err != ErrConflict {
			t.Fatal("old activity sample replaced current state", compact, err)
		}
		report.Users.SampledAt = &at
		report.Users.ActiveIPObservations[0].LastActive = at
		m.peers["node"] = &agentPeer{}
		if err := m.receive(context.Background(), peer, frame()); err != ErrConflict {
			t.Fatal("replaced peer refreshed activity", compact, err)
		}
	}
}
