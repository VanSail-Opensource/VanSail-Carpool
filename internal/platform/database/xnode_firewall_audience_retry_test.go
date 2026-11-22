package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallJournalReloadsAudienceAfterSerializationRetry(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC()
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "other", Name: "Other", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node", "other"} {
		if err := s.ReconcileFirewallMember(ctx, node, true, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	detected := wire.FirewallEvent{ID: fmt.Sprintf("seed-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	if err := s.RecordFirewallEvents(ctx, "node", "seed", []wire.FirewallEvent{detected}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err := s.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 || bans[0].PendingNodes != 2 {
		t.Fatal("missing two-member generation", bans, err)
	}
	for _, query := range []string{
		`CREATE SEQUENCE firewall_audience_retry_attempt`,
		`CREATE FUNCTION retry_firewall_audience_once() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF nextval('firewall_audience_retry_attempt')=1 THEN
  PERFORM pg_advisory_xact_lock(87176382);
  RAISE EXCEPTION 'injected audience retry' USING ERRCODE='40001';
 END IF;
 RETURN NEW; END $$`,
		`CREATE TRIGGER retry_firewall_audience_once BEFORE INSERT ON xnode_firewall_cursors
 FOR EACH ROW EXECUTE FUNCTION retry_firewall_audience_once()`,
	} {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	hold, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hold.Rollback() }()
	if _, err := hold.ExecContext(ctx, `SELECT pg_advisory_xact_lock(87176382)`); err != nil {
		t.Fatal(err)
	}
	expiry := wire.FirewallEvent{ID: fmt.Sprintf("drain-%020d", 1), Kind: "expired", Prefix: bans[0].Prefix, Generation: bans[0].Generation, Time: now}
	done := make(chan error, 1)
	finished := false
	defer func() {
		cancel()
		_ = hold.Rollback()
		if !finished {
			if err := <-done; err != nil {
				t.Logf("journal stopped during cleanup: %v", err)
			}
		}
	}()
	go func() {
		done <- s.RecordFirewallEvents(ctx, "node", "drain", []wire.FirewallEvent{expiry}, policy, now)
	}()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var reached bool
		if err := s.db.QueryRowContext(ctx, `SELECT is_called FROM firewall_audience_retry_attempt`).Scan(&reached); err != nil {
			t.Fatal(err)
		}
		if reached {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("journal did not reach the retry barrier", ctx.Err())
		case <-tick.C:
		}
	}
	// The first attempt saw this member active and is blocked after computing
	// partial expiry. Its retry must see the newly committed retirement.
	if err := s.ReconcileFirewallMember(ctx, "other", false, false, 0); err != nil {
		t.Fatal(err)
	}
	if err := hold.Rollback(); err != nil {
		t.Fatal(err)
	}
	err = <-done
	finished = true
	if err != nil {
		t.Fatal("retried journal failed", err)
	}
	if bans, err = s.FirewallBans(ctx, 10); err != nil || len(bans) != 0 {
		t.Fatal("retried expiry reused the failed attempt's audience", bans, err)
	}
	var attempts int
	if err := s.db.QueryRowContext(ctx, `SELECT last_value FROM firewall_audience_retry_attempt`).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatal("expected one serialization retry", attempts, err)
	}
}
