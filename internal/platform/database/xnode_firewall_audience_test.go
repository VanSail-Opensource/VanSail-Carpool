package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallJournalSharesAudienceAndRefreshesItForTheNextTransaction(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	for _, node := range []string{"other", "retired"} {
		if err := s.SaveXNodeNode(ctx, xnode.Node{ID: node, Name: node, ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
			t.Fatal(err)
		}
	}
	for _, node := range []string{"node", "other", "retired"} {
		if err := s.ReconcileFirewallMember(ctx, node, true, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	var detected []wire.FirewallEvent
	for i, ip := range []string{"8.8.8.8", "9.9.9.9", "1.1.1.1"} {
		detected = append(detected, wire.FirewallEvent{ID: fmt.Sprintf("batch-%020d", i+1), Kind: "detected", IP: ip, Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now})
	}
	if err := s.RecordFirewallEvents(ctx, "node", "batch", detected, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err := s.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 3 {
		t.Fatal("detection batch lost bans", bans, err)
	}
	for i, ban := range bans {
		if ban.PendingNodes != 3 {
			t.Fatal("detection batch mutated its shared audience", ban)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_firewall_sources(id,generation,until_at,refresh_seconds,issued_version,participants,expired)
 SELECT $1,generation,until_at,refresh_seconds,issued_version,participants,expired FROM xnode_firewall_bans WHERE generation=$2`, fmt.Sprint("source-", i), ban.Generation); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReconcileFirewallMember(ctx, "retired", false, false, 0); err != nil {
		t.Fatal(err)
	}
	sequence := len(detected)
	report := func() {
		t.Helper()
		var events []wire.FirewallEvent
		for i, ban := range bans {
			events = append(events,
				wire.FirewallEvent{Kind: "expired", Prefix: ban.Prefix, Generation: ban.Generation},
				wire.FirewallEvent{Kind: "source_expired", Rule: fmt.Sprint("source-", i), Generation: ban.Generation})
		}
		events = append(events, events[0]) // A distinct sequence can repeat an expiry in the same frame.
		for i := range events {
			sequence++
			events[i].ID, events[i].Time = fmt.Sprintf("batch-%020d", sequence), now
		}
		if err := s.RecordFirewallEvents(ctx, "node", "batch", events, policy, now); err != nil {
			t.Fatal(err)
		}
	}
	clock := func() (uint64, uint64) {
		t.Helper()
		var version, sources uint64
		if err := s.db.QueryRowContext(ctx, `SELECT version,source_version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&version, &sources); err != nil {
			t.Fatal(err)
		}
		return version, sources
	}
	before, sourceBefore := clock()
	report()
	remaining, err := s.FirewallBans(ctx, 10)
	if err != nil || len(remaining) != 3 {
		t.Fatal("partial expiry released an active participant", remaining, err)
	}
	for _, ban := range remaining {
		if ban.PendingNodes != 1 {
			t.Fatal("shared audience changed between expiry events", ban)
		}
	}
	if version, sources := clock(); version != before || sources != sourceBefore {
		t.Fatal("partial or duplicate expiry advanced a release clock", version, sources)
	}
	// A later transaction must observe retirement, rather than retain the first
	// batch's audience. The reporting participant is already marked expired.
	if err := s.ReconcileFirewallMember(ctx, "other", false, false, 0); err != nil {
		t.Fatal(err)
	}
	report()
	if remaining, err = s.FirewallBans(ctx, 10); err != nil || len(remaining) != 0 {
		t.Fatal("new transaction reused stale membership", remaining, err)
	}
	var sourcesPending int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_sources WHERE released_version=0`).Scan(&sourcesPending); err != nil || sourcesPending != 0 {
		t.Fatal("source batch lost final release", sourcesPending, err)
	}
	after, sourceAfter := clock()
	if after != before+6 || sourceAfter != after {
		t.Fatal("each generation must release exactly once and retain the source clock", before, after, sourceAfter)
	}
	report()
	if version, sources := clock(); version != after || sources != sourceAfter {
		t.Fatal("released generations were changed by later journal sequences", version, sources)
	}
	var cursor int
	if err := s.db.QueryRowContext(ctx, `SELECT sequence FROM xnode_firewall_cursors WHERE node_id='node' AND namespace='batch'`).Scan(&cursor); err != nil || cursor != sequence {
		t.Fatal("no-op expiry lost committed cursor progress", cursor, err)
	}
}
