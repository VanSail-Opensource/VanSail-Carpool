package xnode

import (
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestUpdateReadinessRequiresFreshCurrentRevisionAndSamePeer(t *testing.T) {
	now := time.Now()
	baseline := liveSnapshot{receivedAt: now, hello: wire.AgentHello{Version: "v2.0.18"}, report: wire.TelemetryReport{Ready: true, Synchronized: true, ConfigRevision: 9}}
	for _, test := range []struct {
		name   string
		mutate func(*liveSnapshot, *agentPeer)
		ready  bool
	}{
		{"matching", func(*liveSnapshot, *agentPeer) {}, true},
		{"wrong running version", func(s *liveSnapshot, _ *agentPeer) { s.hello.Version = "v2.0.17" }, false},
		{"upgrade pending", func(s *liveSnapshot, _ *agentPeer) { s.hello.UpgradePending = true }, false},
		{"unready", func(s *liveSnapshot, _ *agentPeer) { s.report.Ready = false }, false},
		{"unsynchronized", func(s *liveSnapshot, _ *agentPeer) { s.report.Synchronized = false }, false},
		{"old config", func(s *liveSnapshot, _ *agentPeer) { s.report.ConfigRevision = 8 }, false},
		{"stale telemetry", func(s *liveSnapshot, _ *agentPeer) { s.receivedAt = now.Add(-16 * time.Second) }, false},
		{"no config", func(_ *liveSnapshot, p *agentPeer) { p.revision = 0 }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer := &agentPeer{revision: 9}
			snapshot := baseline
			test.mutate(&snapshot, peer)
			master := &Master{service: &Service{now: func() time.Time { return now }}, peers: map[string]*agentPeer{"node": peer}, live: map[string]liveSnapshot{"node": snapshot}}
			if got := master.updateReadyPeer("node", "v2.0.18", peer); got != test.ready {
				t.Fatal(got, test.ready)
			}
			if master.updateReadyPeer("node", "v2.0.18", &agentPeer{revision: 9}) {
				t.Fatal("replacement connection inherited an earlier peer's receipt")
			}
		})
	}
}
