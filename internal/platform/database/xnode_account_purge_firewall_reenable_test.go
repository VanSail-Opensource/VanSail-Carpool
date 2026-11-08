package database

import (
	"context"
	"testing"
	"time"
)

func TestPurgedExemptionCannotReturnWhenSameFirewallConnectionReenables(t *testing.T) {
	s, target, jobID, _ := nativePurgeFixture(t)
	ctx := context.Background()
	seedHistoricalFirewallGrant(t, s, target.UserID, "disabled-host-grant", time.Now().UTC())
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	before, err := s.FirewallMember(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkFirewallDelivered(ctx, "node", before.Version, false, true, before.Epoch); err != nil {
		t.Fatal(err)
	}
	// The authenticated host-disabled evidence permits retirement, although
	// native guard state can retain the old bag for a later enable.
	if err = s.ReconcileFirewallMember(ctx, "node", false, false, before.Version); err != nil {
		t.Fatal(err)
	}
	if ready, e := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); e != nil || !ready {
		t.Fatal("inactive host retained false delivery dependency", ready, e)
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM xnode_firewall_exemptions WHERE user_id=$1`, target.UserID); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileFirewallMember(ctx, "node", true, false, before.Version); err != nil {
		t.Fatal(err)
	}
	after, err := s.FirewallMember(ctx, "node")
	if err != nil || !after.Force || after.Epoch <= before.Epoch {
		t.Fatal("same-socket reenable did not require full replacement", after, err)
	}
	page, _, err := s.FirewallPage(ctx, after.Applied, after.Version, true, "", 256)
	if err != nil || len(page.Exemptions) != 0 {
		t.Fatal("full bank restored purged allowance", page, err)
	}
	if err = s.MarkFirewallDelivered(ctx, "node", after.Version, false, true, before.Epoch); err != nil {
		t.Fatal(err)
	}
	stale, err := s.FirewallMember(ctx, "node")
	if err != nil || !stale.Force || stale.Applied != after.Applied {
		t.Fatal("old epoch cleared reenable barrier", stale, err)
	}
	if err = s.MarkFirewallDelivered(ctx, "node", after.Version, false, true, after.Epoch); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileFirewallMember(ctx, "node", true, false, after.Version); err != nil {
		t.Fatal(err)
	}
	current, err := s.FirewallMember(ctx, "node")
	if err != nil || current.Force || current.Epoch != after.Epoch {
		t.Fatal("steady-state reconciliation minted reset epochs", current, err)
	}
}
