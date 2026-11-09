package xnode

import (
	"encoding/json"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallPortRangesRequireAdvertisedCapability(t *testing.T) {
	for _, policy := range []wire.FirewallPolicy{
		{TCPPortRanges: []wire.PortRange{{From: 8000, To: 8100}}},
		{UDPPortRanges: []wire.PortRange{{From: 20000, To: 30000}}},
		{SSHPortRanges: []wire.PortRange{{From: 2222, To: 2223}}},
		{Forward: []wire.Forward{{Port: 20000, PortEnd: 30000, TargetPort: 443}}},
		{Forward: []wire.Forward{{Port: 20000, PortEnd: 20010, TargetPort: 30000, TargetPortEnd: 30010}}},
	} {
		config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Firewall: policy}
		var capability *CapabilityError
		if err := configurationCapabilities(config, nil); !errors.As(err, &capability) || capability.Capability != "native_firewall_port_ranges_v1" {
			t.Fatalf("unsupported range accepted: %v", err)
		}
		if err := configurationCapabilities(config, []string{"native_firewall_port_ranges_v1"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacySingleFirewallPortsDoNotRequireRangeCapability(t *testing.T) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Firewall: wire.FirewallPolicy{
		TCPPorts: []int{443}, UDPPorts: []int{8443}, SSHPorts: []int{22},
		Forward: []wire.Forward{{Port: 8443, TargetPort: 443}},
	}}
	if err := configurationCapabilities(config, nil); err != nil {
		t.Fatal(err)
	}
}
