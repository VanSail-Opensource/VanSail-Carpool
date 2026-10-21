package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestZeroHourUnbanIsGenerationSafeIdempotentAndPreservesExemptions(t *testing.T) {
	store, owner, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	event := wire.FirewallEvent{ID: "release-00000000000000000001", Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	if err := store.RecordFirewallEvents(ctx, "node", "release", []wire.FirewallEvent{event}, wire.FirewallPolicy{Enabled: true, Protect: true}, now); err != nil {
		t.Fatal(err)
	}
	bans, err := store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 1 {
		t.Fatal(bans, err)
	}
	// A positive grant remains immutable and must survive the ban release.
	if _, err = store.GrantAdminFirewallExemption(ctx, owner.UserID, xnode.AdminFirewallExemption{Prefix: "8.8.8.8/32", Hours: 24, Key: "positive"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AdjustBalance(ctx, owner.UserID, 10000, "seed", "", now); err != nil {
		t.Fatal(err)
	}
	paid, err := store.PurchaseFirewallExemption(ctx, owner.UserID, xnode.FirewallPurchase{IP: "8.8.8.8", Hours: 24, Key: "paid"}, 125, now)
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_exemptions WHERE released_version=0`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	input := xnode.AdminFirewallExemption{Prefix: bans[0].Prefix, Generation: "stale", Hours: 0, Key: "zero"}
	if _, err = store.GrantAdminFirewallExemption(ctx, owner.UserID, input, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale generation accepted", err)
	}
	input.Generation = bans[0].Generation
	receipt, err := store.GrantAdminFirewallExemption(ctx, owner.UserID, input, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.GrantAdminFirewallExemption(ctx, owner.UserID, input, now)
	if err != nil || replay.ID != receipt.ID {
		t.Fatal("release replay", replay, err)
	}
	var after int
	if err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_exemptions WHERE released_version=0`).Scan(&after); err != nil || before != after {
		t.Fatal("release changed grants", before, after, err)
	}
	var paidPrice int64
	if err = store.db.QueryRowContext(ctx, `SELECT price_minor FROM xnode_firewall_exemptions WHERE id=? AND grant_kind='member' AND released_version=0`, paid.ID).Scan(&paidPrice); err != nil || paidPrice != 125 {
		t.Fatal("paid evidence changed", paidPrice, err)
	}
	if done, err := store.FinalizeAdminFirewallExemption(ctx, receipt.ID, now); err != nil || done {
		t.Fatal("completed before delivery", done, err)
	}
	member, err := store.FirewallMember(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.MarkFirewallDelivered(ctx, "node", member.Version, false, true, member.Epoch); err != nil {
		t.Fatal(err)
	}
	if done, err := store.FinalizeAdminFirewallExemption(ctx, receipt.ID, now); err != nil || !done {
		t.Fatal("acknowledgment not observed", done, err)
	}
	event.ID = "release-00000000000000000002"
	if err = store.RecordFirewallEvents(ctx, "node", "release", []wire.FirewallEvent{event}, wire.FirewallPolicy{Enabled: true, Protect: true}, now); err != nil {
		t.Fatal(err)
	}
	input.Key = "old-generation-new-key"
	if _, err = store.GrantAdminFirewallExemption(ctx, owner.UserID, input, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("old generation released a successor ban", err)
	}
}
