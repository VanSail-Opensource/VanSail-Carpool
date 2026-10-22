package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallThresholdAggregatesNodesAndDeduplicatesDeltas(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.SaveXNodeNode(ctx, xnode.Node{ID: "node-two", Name: "Second", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"node", "node-two"} {
		if err := store.ReconcileFirewallMember(ctx, id, true, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true, Rules: map[string]wire.FirewallRule{"closed_port": {Enabled: true, Threshold: 3, WindowSeconds: 60}}}
	first := wire.FirewallEvent{ID: fmt.Sprintf("node-a-events-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "closed_port", Count: 1, FirstSeen: now, Time: now}
	for range 2 {
		if err := store.RecordFirewallEvents(ctx, "node", "node-a-events", []wire.FirewallEvent{first}, policy, now); err != nil {
			t.Fatal(err)
		}
	}
	if bans, err := store.FirewallBans(ctx, 10); err != nil || len(bans) != 0 {
		t.Fatal("per-node threshold or replay caused early ban", bans, err)
	}
	second := first
	second.ID = fmt.Sprintf("node-b-events-%020d", 1)
	second.Count = 2
	second.FirstSeen = now.Add(-time.Millisecond)
	second.Time = second.FirstSeen
	if err := store.RecordFirewallEvents(ctx, "node-two", "node-b-events", []wire.FirewallEvent{second}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err := store.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 || bans[0].Count != "3" || bans[0].PendingNodes != 2 {
		t.Fatalf("fleet threshold: %+v %v", bans, err)
	}
	protected := first
	protected.ID = fmt.Sprintf("node-a-events-%020d", 2)
	protected.IP = "9.9.9.9"
	protected.Count = 100
	if err = store.RecordFirewallEvents(ctx, "node", "node-a-events", []wire.FirewallEvent{protected}, policy, now, []string{protected.IP}); err != nil {
		t.Fatal(err)
	}
	bans, err = store.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 {
		t.Fatal("managed IP banned", bans, err)
	}
}

func TestFirewallDetectionRollbackAndWindowExpiry(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true, Rules: map[string]wire.FirewallRule{"closed_port": {Enabled: true, Threshold: 3, WindowSeconds: 10}}}
	event := wire.FirewallEvent{ID: fmt.Sprintf("window-events-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "closed_port", Count: 2, FirstSeen: now, Time: now}
	bad := event
	bad.ID = fmt.Sprintf("window-events-%020d", 2)
	bad.Kind = "invalid-kind"
	if err := store.RecordFirewallEvents(ctx, "node", "window-events", []wire.FirewallEvent{event, bad}, policy, now); err == nil {
		t.Fatal("invalid batch committed")
	}
	if err := store.RecordFirewallEvents(ctx, "node", "window-events", []wire.FirewallEvent{event}, policy, now); err != nil {
		t.Fatal(err)
	}
	if bans, err := store.FirewallBans(ctx, 10); err != nil || len(bans) != 0 {
		t.Fatal("rolled back memory counted twice", bans, err)
	}
	event.ID = fmt.Sprintf("window-events-%020d", 2)
	event.Time = now.Add(11 * time.Second)
	event.FirstSeen = event.Time
	if err := store.RecordFirewallEvents(ctx, "node", "window-events", []wire.FirewallEvent{event}, policy, event.Time); err != nil {
		t.Fatal(err)
	}
	if bans, err := store.FirewallBans(ctx, 10); err != nil || len(bans) != 0 {
		t.Fatal("expired window contributed", bans, err)
	}
	event.ID = fmt.Sprintf("window-events-%020d", 3)
	event.Count = 1
	if err := store.RecordFirewallEvents(ctx, "node", "window-events", []wire.FirewallEvent{event}, policy, event.Time); err != nil {
		t.Fatal(err)
	}
	if bans, err := store.FirewallBans(ctx, 10); err != nil || len(bans) != 1 || bans[0].Count != "3" {
		t.Fatal("fresh window did not ban", bans, err)
	}
}
