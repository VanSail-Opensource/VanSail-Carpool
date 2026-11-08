package database

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccountPurgeFirewallReleaseIsScopedDurableAndWaitsForDelivery(t *testing.T) {
	ctx := context.Background()
	s, target, jobID, _ := nativePurgeFixture(t)
	at := time.Now().UTC()
	other := createTestUser(t, s, 497681)
	seedHistoricalFirewallGrant(t, s, target.UserID, "purge-shared", at)
	seedHistoricalFirewallGrant(t, s, target.UserID, "purge-only", at)
	seedHistoricalFirewallGrant(t, s, other.ID, "sibling-shared", at)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_firewall_exemptions SET ip='9.9.9.9' WHERE id='purge-only'`); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	member, err := s.FirewallMember(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkFirewallDelivered(ctx, "node", member.Version, false, true, member.Epoch); err != nil {
		t.Fatal(err)
	}
	before := member.Version
	if ready, err := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); err != nil || ready {
		t.Fatal("undelivered exemption completed", ready, err)
	}
	version, err := s.FirewallRevision(ctx)
	if err != nil || version != before+1 {
		t.Fatal(version, err)
	}
	if ready, err := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); err != nil || ready {
		t.Fatal(ready, err)
	}
	if replay, err := s.FirewallRevision(ctx); err != nil || replay != version {
		t.Fatal("replay minted removal revision", replay, err)
	}
	var count int
	var price int64
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*),SUM(price_minor) FROM xnode_firewall_exemptions WHERE user_id=$1 AND released_version=$2`, target.UserID, version).Scan(&count, &price); err != nil || count != 2 || price != 250 {
		t.Fatal("grant evidence was removed", count, price, err)
	}
	var sibling uint64
	if err = s.DB().QueryRowContext(ctx, `SELECT released_version FROM xnode_firewall_exemptions WHERE id='sibling-shared'`).Scan(&sibling); err != nil || sibling != 0 {
		t.Fatal("unrelated exemption retired", sibling, err)
	}
	page, _, err := s.FirewallPage(ctx, before, version, false, "", 256)
	if err != nil || !slices.Contains(page.RemoveExemptions, "9.9.9.9") || slices.Contains(page.RemoveExemptions, "8.8.8.8") {
		t.Fatal("shared prefix removed or private revoke missing", page, err)
	}
	check := func() error {
		_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) { return true, accountPurgeExemptionsReadyTx(ctx, tx, target.UserID) })
		return err
	}
	if err = check(); !errors.Is(err, xnode.ErrAccessAccountingPending) {
		t.Fatal("final scrub bypassed delivery", err)
	}
	if err = s.MarkFirewallDelivered(ctx, "node", version, false, true, member.Epoch+1); err != nil {
		t.Fatal(err)
	}
	if ready, err := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); err != nil || ready {
		t.Fatal("foreign connection epoch became delivery proof", ready, err)
	}
	if err = s.MarkFirewallDelivered(ctx, "node", version, false, true, member.Epoch); err != nil {
		t.Fatal(err)
	}
	if ready, err := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); err != nil || !ready {
		t.Fatal(ready, err)
	}
	if err = check(); err != nil {
		t.Fatal(err)
	}
}

func TestAccountPurgeFirewallChecksHistoricalReleasedRowsAndAdministratorFence(t *testing.T) {
	ctx := context.Background()
	s, target, jobID, _ := nativePurgeFixture(t)
	at := time.Now().UTC()
	seedHistoricalFirewallGrant(t, s, target.UserID, "old-released", at)
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	var version uint64
	if err := s.DB().QueryRowContext(ctx, `UPDATE xnode_firewall_clock SET version=version+1 WHERE singleton=1 RETURNING version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_firewall_exemptions SET released_version=$1 WHERE id='old-released'`, version); err != nil {
		t.Fatal(err)
	}
	if ready, err := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); err != nil || ready {
		t.Fatal("historical pending revoke forgotten", ready, err)
	}
	if got, err := s.FirewallRevision(ctx); err != nil || got != version {
		t.Fatal(got, err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, target.UserID); err != nil {
		t.Fatal(err)
	}
	if ready, err := s.ReleaseAccountPurgeExemptions(ctx, target, jobID); err != nil || ready {
		t.Fatal("administrator guard ignored", ready, err)
	}
	if got, err := s.FirewallRevision(ctx); err != nil || got != version {
		t.Fatal("protected call mutated firewall clock", got, err)
	}
}
