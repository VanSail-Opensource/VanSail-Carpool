package xnode

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestFirewallPongsAreDeliveredOnlyToMatchingTransactions(t *testing.T) {
	peer := &agentPeer{}
	first, ok, _ := peer.watchFirewallPong("first")
	if !ok {
		t.Fatal("missing first watcher")
	}
	second, ok, _ := peer.watchFirewallPong("second")
	if !ok {
		t.Fatal("missing second watcher")
	}
	if _, ok, _ = peer.watchFirewallPong("first"); ok {
		t.Fatal("duplicate not coalesced")
	}
	peer.firewallPong("second")
	select {
	case <-first:
		t.Fatal("pong delivered to wrong transaction")
	default:
	}
	select {
	case <-second:
	default:
		t.Fatal("matching pong missing")
	}
	peer.firewallPong("first")
	select {
	case <-first:
	default:
		t.Fatal("matching pong missing")
	}
	for i := 0; i < 6; i++ {
		if _, added, full := peer.watchFirewallPong(fmt.Sprint(i)); !added || full {
			t.Fatal("watcher rejected before capacity")
		}
	}
	if _, added, full := peer.watchFirewallPong("overflow"); added || !full {
		t.Fatal("watcher capacity was not enforced")
	}
}

func TestFirewallConfirmationRequiresMatchingConfirmedStateAndPeer(t *testing.T) {
	for _, test := range []struct {
		raw, id string
		want    bool
	}{
		{`{"id":"a","status":"confirmed","deadline":"2026-11-02T00:00:00Z","policy":{}}`, "a", true},
		{`{"id":"a","status":"pending"}`, "a", false},
		{`{"id":"a","status":"rolled_back"}`, "a", false},
		{`{"id":"other","status":"confirmed"}`, "a", false},
		{`{`, "a", false},
	} {
		if got := confirmedFirewallStatus(json.RawMessage(test.raw), test.id); got != test.want {
			t.Fatalf("%s: %v", test.raw, got)
		}
	}
	old := &agentPeer{nodeID: "node"}
	peer := &agentPeer{nodeID: "node"}
	m := &Master{peers: map[string]*agentPeer{"node": peer}, live: map[string]liveSnapshot{"node": {firewallPending: "new"}}}
	if m.clearFirewallPending(old, "new") || m.clearFirewallPending(peer, "old") {
		t.Fatal("stale confirmation cleared guard")
	}
	if !m.clearFirewallPending(peer, "new") || m.live["node"].firewallPending != "" {
		t.Fatal("confirmed guard did not clear")
	}
}
