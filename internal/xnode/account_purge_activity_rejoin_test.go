package xnode

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPurgeActivityNewConnectionAuthorityDoesNotUseAgentClock(t *testing.T) {
	at := time.Now().UTC()
	s := NewService(nil, nil, nil, nil, nil)
	s.now = func() time.Time { return at }
	m := NewMaster(s, nil)
	if err := m.fenceAccountActivity([]PurgeActivityScope{{10, "old"}}); err != nil {
		t.Fatal(err)
	}
	until := at.Add(time.Hour)
	config := wire.Config{Users: []wire.User{{ID: 10, AccountingPeriod: "new", InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"vless": json.RawMessage(`{}`)}, ValidUntil: &until}}}
	oldPeer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "same-boot", activityReports: true}
	m.peers["node"] = oldPeer
	m.recordWhitelistState(oldPeer, config, 3)
	newPeer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "same-boot", activityReports: true}
	m.peers["node"] = newPeer
	skewed := at.Add(-time.Minute)
	report := wire.TelemetryReport{ConfigRevision: 3, Ready: true, Synchronized: true, Users: wire.UserMetrics{SampledAt: &at, Connections: map[int]int{10: 1}, ActiveIPObservations: []wire.IPActivity{{UserID: 10, IP: "8.8.4.4", LastActive: skewed}}}}
	if got := m.filterPurgeActivityLocked(newPeer, report); got.Users.Connections[10] != 0 {
		t.Fatal("reconnect inherited prior socket authority")
	}
	m.recordWhitelistState(newPeer, config, 4)
	report.ConfigRevision = 4
	got := m.filterPurgeActivityLocked(newPeer, report)
	if got.Users.Connections[10] != 1 || len(got.Users.ActiveIPObservations) != 1 {
		t.Fatal("legitimate new scope rejected by wall-clock cutoff")
	}
	// Preserved presentation history alone is not fresh native activity and must
	// not create an automatic firewall lease after a new authorization scope.
	_ = m.observeAutomaticActivityLocked(newPeer, got, nil)
	if m.automaticLeases.leases[netip.MustParseAddr("8.8.4.4")] != nil {
		t.Fatal("old historical IP minted a new automatic lease")
	}
}
