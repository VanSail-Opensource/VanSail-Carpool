package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAdminCIDRGrantIsFreeIdempotentAndWaitsForDelivery(t *testing.T) {
	store, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	input := xnode.AdminFirewallExemption{Prefix: "8.8.8.0/24", Hours: 2, Key: "admin-cidr", Reason: "recovery"}
	before, err := store.Balance(ctx, identity.UserID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.GrantAdminFirewallExemption(ctx, identity.UserID, input, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.GrantAdminFirewallExemption(ctx, identity.UserID, input, now)
	if err != nil || replay.ID != receipt.ID {
		t.Fatal("grant replay changed", err)
	}
	conflict := input
	conflict.Hours = 3
	if _, err = store.GrantAdminFirewallExemption(ctx, identity.UserID, conflict, now); !errors.Is(err, ErrConflict) {
		t.Fatal("changed intent replayed", err)
	}
	after, err := store.Balance(ctx, identity.UserID)
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
