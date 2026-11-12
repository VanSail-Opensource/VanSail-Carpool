package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallRecoveryPreservesFailedHistoryAndActiveClaims(t *testing.T) {
	for _, bank := range []string{"dynamic", "sources", "geoip"} {
		for _, status := range []string{"failed", "pending", "processing"} {
			t.Run(bank+"/"+status, func(t *testing.T) {
				s, _, _ := xnodeTrafficFixture(t)
				ctx := context.Background()
				if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
					t.Fatal(err)
				}
				if err := queueRecoveryBank(ctx, s, bank); err != nil {
					t.Fatal(err)
				}
				payload := fmt.Sprintf(`{"bank":%q,"nodeId":"node"}`, bank)
				next := stamp(time.Now().UTC().Add(time.Hour))
				if _, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET status=$1,attempts=7,claim_generation=9,
				 available_at=$2,last_error='original failure' WHERE kind=$3 AND payload=$4`, status, next, xnode.FirewallJob, payload); err != nil {
					t.Fatal(err)
				}
				addFirewallRecoveryFailure(t, s, "older-failure", payload)
				addFirewallRecoveryFailure(t, s, "newer-failure", payload)
				before := firewallRecoveryRows(t, s)
				for range 3 {
					if err := queueRecoveryBank(ctx, s, bank); err != nil {
						t.Fatal("historical failures blocked recovery", err)
					}
				}
				after := firewallRecoveryRows(t, s)
				for id, original := range before {
					if after[id] != original {
						t.Fatalf("reconciliation rewrote history, another bank or active claim %s", id)
					}
				}
				var active int
				if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1 AND payload=$2 AND status IN ('pending','processing')`, xnode.FirewallJob, payload).Scan(&active); err != nil || active != 1 {
					t.Fatalf("active deliveries = %d, err=%v", active, err)
				}
			})
		}
	}
}

func TestFirewallRecoveryCoalescesConcurrentEnqueues(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10 WHERE kind=$1`, xnode.FirewallJob); err != nil {
		t.Fatal(err)
	}
	before := firewallRecoveryRows(t, s)
	results := make(chan error, 8)
	start := make(chan struct{})
	for range cap(results) {
		go func() {
			<-start
			results <- s.ReconcileFirewallMember(ctx, "node", true, false, 0)
		}()
	}
	close(start)
	for range cap(results) {
		if err := <-results; err != nil {
			t.Error("concurrent recovery failed", err)
		}
	}
	var active int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1 AND status='pending'`, xnode.FirewallJob).Scan(&active); err != nil || active != 2 {
		t.Fatalf("expected one active delivery per bank, got %d, err=%v", active, err)
	}
	after := firewallRecoveryRows(t, s)
	for id, original := range before {
		if after[id] != original {
			t.Fatalf("concurrent recovery rewrote failed receipt %s", id)
		}
	}
}

func TestFirewallJournalCommitsWithFailedAndActiveDelivery(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	addFirewallRecoveryFailure(t, s, "failed-dynamic", `{"bank":"dynamic","nodeId":"node"}`)
	now := time.Now().UTC()
	policy := wire.FirewallPolicy{Enabled: true, Protect: true, Rules: map[string]wire.FirewallRule{"closed_port": {Enabled: true, Threshold: 1, WindowSeconds: 60}}}
	event := wire.FirewallEvent{ID: fmt.Sprintf("recovery-events-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "closed_port", Count: 1, FirstSeen: now, Time: now}
	for range 2 {
		if err := s.RecordFirewallEvents(ctx, "node", "recovery-events", []wire.FirewallEvent{event}, policy, now); err != nil {
			t.Fatal("historical delivery failure blocked event commit", err)
		}
	}
	var cursor int
	if err := s.db.QueryRowContext(ctx, `SELECT sequence FROM xnode_firewall_cursors WHERE node_id='node' AND namespace='recovery-events'`).Scan(&cursor); err != nil || cursor != 1 {
		t.Fatalf("event cursor = %d, err=%v", cursor, err)
	}
	bans, err := s.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 1 {
		t.Fatalf("replayed detection = %+v, err=%v", bans, err)
	}
}

func queueRecoveryBank(ctx context.Context, s *Store, bank string) error {
	if bank == "geoip" {
		return s.QueueXNodeGeoIP(ctx, "node")
	}
	return s.ReconcileFirewallMember(ctx, "node", true, false, 0)
}

func addFirewallRecoveryFailure(t *testing.T, s *Store, id, payload string) {
	t.Helper()
	now := stamp(time.Now().UTC())
	if _, err := s.db.Exec(`INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,last_error,created_at,updated_at,claim_generation)
	 VALUES($1,$2,$3,'failed',10,$4,'native command failed',$4,$4,11)`, id, xnode.FirewallJob, payload, now); err != nil {
		t.Fatal(err)
	}
}

func firewallRecoveryRows(t *testing.T, s *Store) map[string]string {
	t.Helper()
	rows, err := s.db.Query(`SELECT id,row_to_json(job)::text FROM outbox_jobs job WHERE kind=$1`, xnode.FirewallJob)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		result[id] = raw
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
