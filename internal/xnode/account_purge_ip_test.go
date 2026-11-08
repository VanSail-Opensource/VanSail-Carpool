package xnode

import (
	"encoding/json"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPurgeIPRequiresExactPositiveCurrentlyAppliedPrincipal(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil)
	m := NewMaster(s, nil)
	control := AccountPurgeIPControl{AgentID: "agent"}
	control.Block.NodeUUID = "node"
	peer := &agentPeer{nodeID: "node", agentID: "agent", revision: 7, digest: "applied"}
	peer.wireState = &preparedWire{digest: "applied", config: wire.Config{Users: []wire.User{{ID: 42, AccountingPeriod: "old-period", InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"vless": json.RawMessage(`{"id":"owned"}`)}}}}}
	m.peers["node"] = peer
	m.live["node"] = liveSnapshot{report: wire.TelemetryReport{ConfigRevision: 7, Ready: true, Synchronized: true}}
	if m.accountPurgeIPPeer(control, 42) != peer {
		t.Fatal("current owned positive subject unavailable")
	}
	for _, id := range []int64{0, -1, 43, 1 << 53} {
		if m.accountPurgeIPPeer(control, id) != nil {
			t.Fatal("global/foreign/out-of-range subject accepted", id)
		}
	}
	peer.wireState.digest = "not-applied"
	if m.accountPurgeIPPeer(control, 42) != nil {
		t.Fatal("prepared but unapplied configuration authorized control")
	}
	peer.wireState.digest = "applied"
	control.AgentID = "replaced-agent"
	if m.accountPurgeIPPeer(control, 42) != nil {
		t.Fatal("another agent inherited old subject")
	}
	control.AgentID = "agent"
	peer.wireState.config.Users = nil
	if m.accountPurgeIPPeer(control, 42) != nil {
		t.Fatal("removed subject was reconstructed for unblocking")
	}
}
