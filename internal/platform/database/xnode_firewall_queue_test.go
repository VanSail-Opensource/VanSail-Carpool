package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func clearDeliveredFirewallJobs(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	m, err := s.FirewallMember(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	for _, sources := range []bool{true, false} {
		v := m.Version
		if sources {
			v = m.SourceVersion
		}
		if err = s.MarkFirewallDelivered(ctx, "node", v, sources, true, m.Epoch); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.db.Exec(`DELETE FROM outbox_jobs WHERE kind=$1`, xnode.FirewallJob); err != nil {
		t.Fatal(err)
	}
}

func TestFirewallJournalQueuesOnlyDesiredChangesAndLaggingBanks(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	clearDeliveredFirewallJobs(t, s)
	policy := wire.FirewallPolicy{Enabled: true, Protect: true, Rules: map[string]wire.FirewallRule{"closed_port": {Enabled: true, Threshold: 2, WindowSeconds: 60}}}
	event := wire.FirewallEvent{ID: fmt.Sprintf("queue-events-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "closed_port", Count: 1, FirstSeen: now, Time: now}
	count := func(want int) {
		t.Helper()
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1`, xnode.FirewallJob).Scan(&n); err != nil || n != want {
			t.Fatalf("firewall jobs=%d, want=%d, error=%v", n, want, err)
		}
	}
	for range 2 {
		if err := s.RecordFirewallEvents(ctx, "node", "queue-events", []wire.FirewallEvent{event}, policy, now); err != nil {
			t.Fatal(err)
		}
		count(0)
	}
	event.ID = fmt.Sprintf("queue-events-%020d", 2)
	if err := s.RecordFirewallEvents(ctx, "node", "queue-events", []wire.FirewallEvent{event}, policy, now); err != nil {
		t.Fatal(err)
	}
	count(1) // A new dynamic ban does not requeue the current source bank.
	var bank string
	if err := s.db.QueryRow(`SELECT (CASE ((payload)::jsonb #> '{bank}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{bank}' END) FROM outbox_jobs WHERE kind=$1`, xnode.FirewallJob).Scan(&bank); err != nil || bank != "dynamic" {
		t.Fatal("wrong bank queued", bank, err)
	}
	// Even while delivery is behind, replaying the already committed event
	// must not regenerate fleet work. Periodic reconciliation owns recovery.
	if _, err := s.db.Exec(`DELETE FROM outbox_jobs WHERE kind=$1`, xnode.FirewallJob); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordFirewallEvents(ctx, "node", "queue-events", []wire.FirewallEvent{event}, policy, now); err != nil {
		t.Fatal(err)
	}
	count(0)
	if err := s.ReconcileFirewallMember(ctx, "node", true, false, 0); err != nil {
		t.Fatal(err)
	}
	count(1)
}

func TestFirewallReconciliationPreservesPendingRetryAndReconnectsBothBanks(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	clearDeliveredFirewallJobs(t, s)
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	next := stamp(time.Now().UTC().Add(time.Hour))
	if _, err := s.db.Exec(`UPDATE outbox_jobs SET attempts=4,available_at=$1 WHERE kind=$2`, next, xnode.FirewallJob); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.ReconcileFirewallMember(ctx, "node", true, false, 0); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1 AND attempts=4 AND available_at=$2`, xnode.FirewallJob, next).Scan(&count); err != nil || count != 2 {
		t.Fatal("reconciliation erased retry delay or missed reconnect bank", count, err)
	}
}

func TestFirewallExpiryQueuesFinalReleaseWithoutAnotherReconciliation(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "other", Name: "Other", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node", "other"} {
		if err := s.ReconcileFirewallMember(ctx, node, true, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	detected := wire.FirewallEvent{ID: fmt.Sprintf("queued-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	if err := s.RecordFirewallEvents(ctx, "node", "queued", []wire.FirewallEvent{detected}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err := s.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 {
		t.Fatal("missing queued ban", bans, err)
	}
	for _, node := range []string{"node", "other"} {
		member, err := s.FirewallMember(ctx, node)
		if err != nil {
			t.Fatal(err)
		}
		for _, sources := range []bool{false, true} {
			version := member.Version
			if sources {
				version = member.SourceVersion
			}
			if err := s.MarkFirewallDelivered(ctx, node, version, sources, true, member.Epoch); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=$1`, xnode.FirewallJob); err != nil {
		t.Fatal(err)
	}
	for i, node := range []string{"node", "node", "other"} {
		expiry := wire.FirewallEvent{ID: fmt.Sprintf("queued-%020d", i+2), Kind: "expired", Prefix: bans[0].Prefix, Generation: bans[0].Generation, Time: now}
		if err := s.RecordFirewallEvents(ctx, node, "queued", []wire.FirewallEvent{expiry}, policy, now); err != nil {
			t.Fatal(err)
		}
		want := 0
		if node == "other" {
			want = 2 // Final release must queue the dynamic bank for both active members.
		}
		var dynamic, total int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE payload::jsonb->>'bank'='dynamic'),COUNT(*) FROM outbox_jobs WHERE kind=$1 AND status='pending'`, xnode.FirewallJob).Scan(&dynamic, &total); err != nil || dynamic != want || total != want {
			t.Fatal("expiry queued unrelated work or lost its final release", i, dynamic, total, err)
		}
	}
}
