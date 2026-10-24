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
	if _, err = s.db.Exec(`DELETE FROM outbox_jobs WHERE kind=?`, xnode.FirewallJob); err != nil {
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
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind=?`, xnode.FirewallJob).Scan(&n); err != nil || n != want {
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
	if err := s.db.QueryRow(`SELECT json_extract(payload,'$.bank') FROM outbox_jobs WHERE kind=?`, xnode.FirewallJob).Scan(&bank); err != nil || bank != "dynamic" {
		t.Fatal("wrong bank queued", bank, err)
	}
	// Even while delivery is behind, replaying the already committed event
	// must not regenerate fleet work. Periodic reconciliation owns recovery.
	if _, err := s.db.Exec(`DELETE FROM outbox_jobs WHERE kind=?`, xnode.FirewallJob); err != nil {
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
	if _, err := s.db.Exec(`UPDATE outbox_jobs SET attempts=4,available_at=? WHERE kind=?`, next, xnode.FirewallJob); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.ReconcileFirewallMember(ctx, "node", true, false, 0); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND attempts=4 AND available_at=?`, xnode.FirewallJob, next).Scan(&count); err != nil || count != 2 {
		t.Fatal("reconciliation erased retry delay or missed reconnect bank", count, err)
	}
}
