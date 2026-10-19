package xnode

import (
	"context"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallEventFrameLimitIsEnforcedBeforePersistence(t *testing.T) {
	master := &Master{live: map[string]liveSnapshot{
		"node": {hello: wire.AgentHello{Capabilities: []string{"firewall_control_v1"}}},
	}}
	peer := &agentPeer{nodeID: "node"}
	events := make([]wire.FirewallEvent, wire.MaxFirewallEventsPerFrame+1)
	message := wire.New("firewall.events", "", 0, wire.FirewallEvents{InstanceID: "events", Events: events})
	err := master.receiveFirewallEvents(context.Background(), peer, message)
	var coded interface{ frameErrorCode() string }
	if !errors.As(err, &coded) || coded.frameErrorCode() != "XNODE_FIREWALL_EVENT_FRAME_OVERSIZED" {
		t.Fatalf("oversized frame error = %v", err)
	}
}

func TestFirewallEventFrameErrorCodesAreSafe(t *testing.T) {
	secret := errors.New("signed url?token=must-not-be-logged")
	err := firewallEventFailure("XNODE_FIREWALL_EVENT_STORAGE_FAILED", secret)
	if got := safeFrameErrorCode(err); got != "XNODE_FIREWALL_EVENT_STORAGE_FAILED" {
		t.Fatalf("safe code = %q", got)
	}
	if got := safeFrameErrorCode(errors.New("ordinary rejection")); got != "XNODE_FRAME_REJECTED" {
		t.Fatalf("fallback code = %q", got)
	}
}
