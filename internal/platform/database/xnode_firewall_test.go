package database

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

func TestFirewallReplayAndAllNodeExpiry(t *testing.T) {
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
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	event := wire.FirewallEvent{ID: fmt.Sprintf("events-one-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	for range 2 {
		if err := store.RecordFirewallEvents(ctx, "node", "events-one", []wire.FirewallEvent{event}, policy, now); err != nil {
			t.Fatal(err)
		}
	}
	bans, err := store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 1 || bans[0].PendingNodes != 2 {
		t.Fatalf("replayed detection: %+v %v", bans, err)
	}
	generation := bans[0].Generation
	// One node expiring never removes a still participating offline node.
	expiry := wire.FirewallEvent{ID: fmt.Sprintf("events-one-%020d", 2), Kind: "expired", Prefix: "8.8.8.8/32", Generation: generation, Time: now.Add(72 * time.Hour)}
	if err = store.RecordFirewallEvents(ctx, "node", "events-one", []wire.FirewallEvent{expiry}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err = store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 1 || bans[0].PendingNodes != 1 {
		t.Fatalf("early global expiry: %+v %v", bans, err)
	}
	expiry.ID = fmt.Sprintf("events-two-%020d", 1)
	if err = store.RecordFirewallEvents(ctx, "node-two", "events-two", []wire.FirewallEvent{expiry}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err = store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 0 {
		t.Fatalf("global expiry missing: %+v %v", bans, err)
	}
}
