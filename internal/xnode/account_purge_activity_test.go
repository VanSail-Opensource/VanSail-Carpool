package xnode

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPurgeActivityScrubsOnlyOwnedAssociationsAndRejectsOldScope(t *testing.T) {
	at := time.Now().UTC()
	s := NewService(nil, nil, nil, nil, nil)
	s.now = func() time.Time { return at }
	m := NewMaster(s, nil)
	for _, id := range []int64{10, 20} {
		if _, err := m.automaticLeases.manualGrant(id, "8.8.8.8", at); err != nil {
			t.Fatal(err)
		}
	}
	original := wire.UserMetrics{Connections: map[int]int{10: 1, 20: 2}, ActiveIPs: map[int][]string{10: {"8.8.8.8"}, 20: {"8.8.8.8"}}, ActiveIPObservations: []wire.IPActivity{{UserID: 10, IP: "8.8.8.8", LastActive: at}, {UserID: 20, IP: "8.8.8.8", LastActive: at}}}
	m.live["node"] = liveSnapshot{report: wire.TelemetryReport{Users: original}}
	if err := m.fenceAccountActivity([]PurgeActivityScope{{10, "old-paid-period"}}); err != nil {
		t.Fatal(err)
	}
	lease := m.automaticLeases.leases[netip.MustParseAddr("8.8.8.8")]
	if lease == nil || len(lease.owners) != 1 || lease.owners[20].IsZero() == false || m.automaticLeases.ownerAssociations != 1 {
		t.Fatal("shared IP lease/other owner changed", lease, m.automaticLeases.ownerAssociations)
	}
	if _, ok := lease.owners[10]; ok {
		t.Fatal("purged owner association retained")
	}
	if len(m.live["node"].report.Users.ActiveIPObservations) != 1 || m.live["node"].report.Users.Connections[20] != 2 || original.Connections[10] != 1 || len(original.ActiveIPObservations) != 2 {
		t.Fatal("snapshot scrub mutated unrelated/shared banks")
	}
	peer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "boot", activityReports: true}
	m.peers["node"] = peer
	until := at.Add(time.Hour)
	user := wire.User{ID: 10, AccountingPeriod: "old-paid-period", InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"vless": json.RawMessage(`{}`)}, ValidUntil: &until}
	m.recordWhitelistState(peer, wire.Config{Users: []wire.User{user}}, 2)
	report := wire.TelemetryReport{ConfigRevision: 2, Ready: true, Synchronized: true, Users: original}
	if got := m.filterPurgeActivityLocked(peer, report); got.Users.Connections[10] != 0 || got.Users.Connections[20] != 2 {
		t.Fatal("old applied scope repopulated purged activity")
	}
	user.AccountingPeriod = "new-paid-period"
	m.recordWhitelistState(peer, wire.Config{Users: []wire.User{user}}, 3)
	if got := m.filterPurgeActivityLocked(peer, report); got.Users.Connections[10] != 0 {
		t.Fatal("unapplied new scope bypassed fence")
	}
	report.ConfigRevision = 3
	if got := m.filterPurgeActivityLocked(peer, report); got.Users.Connections[10] != 1 {
		t.Fatal("new paid native scope stayed fenced")
	}
	m.recordWhitelistState(peer, wire.Config{}, 4)
	report.ConfigRevision = 4
	if got := m.filterPurgeActivityLocked(peer, report); got.Users.Connections[10] != 0 {
		t.Fatal("revoked successor retained activity authority")
	}
	at = at.Add(2*time.Hour + 31*time.Second)
	_ = m.filterPurgeActivityLocked(peer, report)
	if len(m.purgedActivity) != 0 || m.purgeActivityScopes != 0 {
		t.Fatal("volatile purge scope did not expire")
	}
}
