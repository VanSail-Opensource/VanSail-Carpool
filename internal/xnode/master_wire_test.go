package xnode

import (
	"encoding/json"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestWireProjectionIsImmutableAndCapabilityFailureCannotReuseSentState(t *testing.T) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{{ID: 1, BlockNew: true}}, Firewall: wire.FirewallPolicy{Protect: true}}
	capabilities := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"}
	peer := &agentPeer{compiled: &compiledState{config: config}}
	first, err := preparePeerWire(peer, config, capabilities)
	if err != nil || first.config.Users[0].BlockNew || !config.Users[0].BlockNew || first.config.Firewall.Protect || !config.Firewall.Protect {
		t.Fatal("projection mutated cached state or retained volatile controls", err)
	}
	for range 1000 {
		next, err := preparePeerWire(peer, config, capabilities)
		if err != nil || next != first {
			t.Fatal("unchanged state was rebuilt", err)
		}
	}
	config.BlockedDomains = []string{"domain:example.com"}
	peer.compiled = &compiledState{config: config}
	failed, err := preparePeerWire(peer, config, capabilities)
	if !errors.Is(err, ErrUnavailable) || failed == first || failed.digest != "" {
		t.Fatal("unsupported policy reused previously sent state", err)
	}
	for range 1000 {
		next, err := preparePeerWire(peer, config, capabilities)
		if !errors.Is(err, ErrUnavailable) || next != failed {
			t.Fatal("capability rejection was reprocessed", err)
		}
	}
	peer.compiled = &compiledState{config: config}
	capabilities = append(capabilities, "shared_domain_blocks_v1")
	if next, err := preparePeerWire(peer, config, capabilities); err != nil || next.digest == first.digest {
		t.Fatal("new capable peer failed to adopt changed state", err)
	}
}
