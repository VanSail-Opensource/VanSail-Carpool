package xnode

import (
	"context"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestDDNSReconnectGatesOutboundAndInboundTransitBeforeActivation(t *testing.T) {
	m := NewMaster(NewService(nil, nil, nil, nil, nil), nil)
	m.ddnsProtectedIPs = []string{"9.9.9.9"}
	m.ddnsNodeIPs = map[string][]string{"changed-source": {"9.9.9.9"}}
	capabilities := []string{"firewall_master_observation_v1"}
	m.live["receiver"] = liveSnapshot{hello: wire.AgentHello{Capabilities: capabilities}}
	peer := &agentPeer{nodeID: "receiver"}
	for _, prepared := range []*preparedWire{{routeAddresses: map[string]bool{"9.9.9.9": true}}, {routeSources: map[string]bool{"changed-source": true}}} {
		// No queued bank can succeed in this fixture: activation must stay gated.
		ready, err := m.ddnsRouteProtectionReady(context.Background(), peer, prepared, capabilities)
		if ready || !errors.Is(err, ErrUnavailable) {
			t.Fatal("unprotected route activated", ready, err)
		}
		peer.protectedIPs = []string{"9.9.9.9"}
		if ready, err = m.ddnsRouteProtectionReady(context.Background(), peer, prepared, capabilities); !ready || err != nil {
			t.Fatal("acknowledged bank did not release route", ready, err)
		}
		peer.protectedIPs = nil
		if ready, err = m.ddnsRouteProtectionReady(context.Background(), peer, prepared, nil); !ready || err != nil {
			t.Fatal("legacy authorization compatibility changed", ready, err)
		}
	}
	unrelated := &preparedWire{routeSources: map[string]bool{"other-source": true}}
	if ready, err := m.ddnsRouteProtectionReady(context.Background(), peer, unrelated, capabilities); !ready || err != nil {
		t.Fatal("unrelated paid configuration was delayed", ready, err)
	}
}
