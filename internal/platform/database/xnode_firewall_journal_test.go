package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallJournalDrainsBeforeJoinAndAfterRetirement(t *testing.T) {
	for _, joined := range []bool{false, true} {
		t.Run(fmt.Sprint("previous-member-", joined), func(t *testing.T) {
			store, _, _ := xnodeTrafficFixture(t)
			ctx := context.Background()
			if joined {
				if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
					t.Fatal(err)
				}
				if err := store.ReconcileFirewallMember(ctx, "node", false, false, 0); err != nil {
					t.Fatal(err)
				}
			}
			at := time.Now().UTC()
			events := []wire.FirewallEvent{
				{ID: fmt.Sprintf("draining-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: at, Time: at},
				{ID: fmt.Sprintf("draining-%020d", 2), Kind: "expired", Prefix: "8.8.8.8/32", Generation: "old", Time: at},
				{ID: fmt.Sprintf("draining-%020d", 3), Kind: "source_expired", Rule: "old-source", Generation: "old", Time: at},
			}
			for range 2 {
				if err := store.RecordFirewallEvents(ctx, "node", "draining", events, wire.FirewallPolicy{Enabled: true, Protect: true}, at); err != nil {
					t.Fatal("pending journal rejected", err)
				}
			}
			var cursor uint64
			if err := store.db.QueryRow(`SELECT sequence FROM xnode_firewall_cursors WHERE node_id='node' AND namespace='draining'`).Scan(&cursor); err != nil || cursor != 3 {
				t.Fatalf("cursor = %d, %v", cursor, err)
			}
			bans, err := store.FirewallBans(ctx, 10)
			if err != nil || len(bans) != 0 {
				t.Fatalf("inactive detector authorized bans: %+v, %v", bans, err)
			}
		})
	}
}

func TestFirewallJournalSkipsNonPublicObservationsWithoutBlockingPublicBans(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	var events []wire.FirewallEvent
	for index, ip := range []string{"100.64.0.1", "192.0.2.1", "2001:db8::1", "8.8.8.8"} {
		events = append(events, wire.FirewallEvent{ID: fmt.Sprintf("public-only-%020d", index+1), Kind: "detected", IP: ip, Rule: "xray_tcp_probe", Count: 3, FirstSeen: at, Time: at})
	}
	for range 2 {
		if err := store.RecordFirewallEvents(ctx, "node", "public-only", events, wire.FirewallPolicy{Enabled: true, Protect: true}, at); err != nil {
			t.Fatal(err)
		}
	}
	bans, err := store.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 || bans[0].IP != "8.8.8.8" {
		t.Fatalf("public-only bans = %+v, %v", bans, err)
	}
}
