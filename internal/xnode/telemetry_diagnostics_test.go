package xnode

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestTelemetryRejectionsRetainSafeReasonsAndPreviousSnapshot(t *testing.T) {
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		reason string
		change func(*wire.TelemetryReport, *agentPeer)
	}{
		{"identity", "identity_mismatch", func(r *wire.TelemetryReport, _ *agentPeer) { r.BootID = "private-identity" }},
		{"metrics", "user_metrics", func(r *wire.TelemetryReport, _ *agentPeer) { r.Users.Connections = map[int]int{1: -1} }},
		{"negotiation", "activity_not_negotiated", func(_ *wire.TelemetryReport, p *agentPeer) { p.activityReports = false }},
		{"report clock", "sample_after_report", func(r *wire.TelemetryReport, _ *agentPeer) { r.Time = now.Add(-time.Second) }},
		{"future clock", "sample_in_future", func(r *wire.TelemetryReport, _ *agentPeer) {
			at := now.Add(time.Minute)
			r.Time, r.Users.SampledAt = at, &at
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			master := NewMaster(NewService(nil, nil, nil, nil, nil), slog.New(slog.NewJSONHandler(&output, nil)))
			master.service.now = func() time.Time { return now }
			peer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "boot", activityReports: true}
			master.peers["node"] = peer
			old := now.Add(-time.Minute)
			master.live["node"] = liveSnapshot{receivedAt: old, report: wire.TelemetryReport{ConfigRevision: 9}}
			report := wire.TelemetryReport{AgentID: "agent", BootID: "boot", Time: now, Machine: json.RawMessage(`{"private":"secret-payload"}`), Users: wire.UserMetrics{SampledAt: &now}}
			test.change(&report, peer)
			message := wire.New("telemetry.report", "", 0, report)
			err := master.receiveActivity(peer, message)
			var failure telemetryFrameFailure
			if !errors.Is(err, ErrInvalid) || !errors.As(err, &failure) || failure.reason != test.reason {
				t.Fatalf("diagnostic lost: %v (%+v)", err, failure)
			}
			master.rejectPeerFrame(peer, message, err)
			logged := output.String()
			if !strings.Contains(logged, test.reason) || strings.Contains(logged, "private-identity") || strings.Contains(logged, "secret-payload") {
				t.Fatal("diagnostic missing or exposed payload", logged)
			}
			if got := master.live["node"]; !got.receivedAt.Equal(old) || got.report.ConfigRevision != 9 {
				t.Fatal("invalid frame refreshed or replaced the accepted snapshot")
			}
		})
	}
}

func TestTelemetryDecodeReasonsExcludeRawDecoderErrors(t *testing.T) {
	for _, test := range []struct {
		kind, payload, reason string
		compact               bool
	}{
		{"telemetry.report", `{"private-token":`, "legacy_decode", false},
		{"telemetry.compact", `{}`, "compact_not_negotiated", false},
		{"telemetry.compact", `{"private-token":`, "compact_decode", true},
		{"telemetry.compact", `{}`, "compact_schema", true},
	} {
		t.Run(test.reason, func(t *testing.T) {
			_, err := decodeTelemetryFrame(&agentPeer{compactTelemetry: test.compact}, wire.Message{Type: test.kind, Data: json.RawMessage(test.payload)})
			var failure telemetryFrameFailure
			if !errors.As(err, &failure) || !errors.Is(err, ErrInvalid) || failure.reason != test.reason || strings.Contains(err.Error(), "private-token") {
				t.Fatal("unsafe or incorrect decode reason", err)
			}
		})
	}
}
