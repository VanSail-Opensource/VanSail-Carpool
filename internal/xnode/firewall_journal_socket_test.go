package xnode_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallJournalIsAcknowledgedBeforeSynchronizedTelemetry(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	node, settings, err := f.service.Node(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	settings.Firewall.Enabled, settings.Firewall.Protect = true, true
	if _, err = f.service.SaveNode(ctx, node, settings); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.master)
	defer server.Close()
	_, token, err := f.service.Enrollment(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	socket := agentSocket(t, server, token, fixtureAgent, 0, "firewall_control_v1")
	defer socket.Close()
	at := time.Now().UTC()
	event := wire.FirewallEvent{ID: fmt.Sprintf("early-events-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: at, Time: at}
	if err = socket.WriteJSON(wire.New("firewall.events", "", 0, wire.FirewallEvents{InstanceID: "early-events", Events: []wire.FirewallEvent{event}})); err != nil {
		t.Fatal(err)
	}
	message := readFrame(t, socket, "firewall.events.ack")
	var ack wire.FirewallEventAck
	if err = wire.Decode(message.Data, &ack); err != nil || len(ack.IDs) != 1 || ack.IDs[0] != event.ID {
		t.Fatalf("journal acknowledgment = %+v, %v", ack, err)
	}
	bans, err := f.store.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 {
		t.Fatalf("early journal not committed: %+v, %v", bans, err)
	}
}
