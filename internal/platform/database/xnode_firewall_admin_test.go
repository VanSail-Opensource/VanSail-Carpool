package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestAdminCIDRGrantIsFreeIdempotentAndWaitsForDelivery(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	actor := createTestAdmin(t, store, 95110)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	input := xnode.AdminFirewallExemption{Prefix: "8.8.8.0/24", Hours: 2, Key: "admin-cidr", Reason: "recovery"}
	before, err := store.Balance(ctx, actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.GrantAdminFirewallExemption(ctx, actor.ID, input, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.GrantAdminFirewallExemption(ctx, actor.ID, input, now)
	if err != nil || replay.ID != receipt.ID {
		t.Fatal("grant replay changed", err)
	}
	conflict := input
	conflict.Hours = 3
	if _, err = store.GrantAdminFirewallExemption(ctx, actor.ID, conflict, now); !errors.Is(err, ErrConflict) {
		t.Fatal("changed intent replayed", err)
	}
	after, err := store.Balance(ctx, actor.ID)
	if err != nil || before.Minor != after.Minor {
		t.Fatal("admin grant debited wallet", err)
	}
	if done, err := store.FinalizeAdminFirewallExemption(ctx, receipt.ID, now); err != nil || done {
		t.Fatal("undelivered grant completed", err)
	}
	member, err := store.FirewallMember(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.MarkFirewallDelivered(ctx, "node", member.Version, false, true, member.Epoch); err != nil {
		t.Fatal(err)
	}
	if done, err := store.FinalizeAdminFirewallExemption(ctx, receipt.ID, now); err != nil || !done {
		t.Fatal("delivered grant stayed pending", err)
	}
	op, err := store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil || op.Receipt.Status != "succeeded" {
		t.Fatal("receipt missing", err)
	}
	if done, err := store.FinalizeAdminFirewallExemption(ctx, receipt.ID, now.Add(3*time.Hour)); err != nil || !done {
		t.Fatal("terminal receipt was rewritten", err)
	}
}

func TestFirewallAdministratorDemotionBlocksFreshGrantsAndReleases(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	actor := createTestAdmin(t, store, 95113)
	ctx, now := context.Background(), time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	event := wire.FirewallEvent{ID: "admin-00000000000000000001", Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	if err := store.RecordFirewallEvents(ctx, "node", "admin", []wire.FirewallEvent{event}, wire.FirewallPolicy{Enabled: true, Protect: true}, now); err != nil {
		t.Fatal(err)
	}
	bans, err := store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 1 {
		t.Fatal(bans, err)
	}
	input := xnode.AdminFirewallExemption{Prefix: "9.9.9.9/32", Hours: 2, Key: "accepted"}
	accepted, err := store.GrantAdminFirewallExemption(ctx, actor.ID, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	for _, fresh := range []xnode.AdminFirewallExemption{
		{Prefix: "1.1.1.1/32", Hours: 1, Key: "denied-grant"},
		{Prefix: bans[0].Prefix, Generation: bans[0].Generation, Hours: 0, Key: "denied-release"},
	} {
		if _, err = store.GrantAdminFirewallExemption(ctx, actor.ID, fresh, now); !errors.Is(err, ErrConflict) {
			t.Fatal("demoted administrator mutated the fleet firewall", err)
		}
	}
	if replay, err := store.GrantAdminFirewallExemption(ctx, actor.ID, input, now); err != nil || replay.ID != accepted.ID {
		t.Fatal("accepted intent lost replay after demotion", replay, err)
	}
	if bans, err = store.FirewallBans(ctx, 100); err != nil || len(bans) != 1 {
		t.Fatal("rejected release changed active ban evidence", bans, err)
	}
	var count int
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE kind=$1`, xnode.FirewallExemptionOperation).Scan(&count); err != nil || count != 1 {
		t.Fatal("rejected firewall request retained a durable intent", count, err)
	}
}
