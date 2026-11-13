package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPricingHealthSerializesIndependentRecorders(t *testing.T) {
	t.Parallel()
	store, _, _ := xnodeTrafficFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	blocker, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err := blocker.ExecContext(ctx, `SELECT id FROM xnode_nodes WHERE id='node' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	results := make(chan error, 2)
	for range 2 {
		independent := NewStore(store.DB())
		go func() {
			results <- independent.RecordPricingHealth(ctx, pricing.Health{NodeID: "node", State: "OFFLINE", At: at})
		}()
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
			AND strpos(query,'SELECT id FROM xnode_nodes WHERE id=$1 FOR UPDATE')>0`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("recorders did not wait on the shared node lock")
		}
	}
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	var incidents, notices int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_operational_incidents WHERE scope_id='node' AND ended_at IS NULL`).Scan(&incidents); err != nil || incidents != 1 {
		t.Fatalf("open incidents = %d, %v; want one", incidents, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='pricing_health_notice'`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("incident notices = %d, %v; want one", notices, err)
	}
}

func TestPricingHealthClosesLegacyDuplicatesAndIgnoresOlderTransitions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _, _ := xnodeTrafficFixture(t)
	at := time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO pricing_operational_incidents
		(id,scope_type,scope_id,state,started_at,reason) VALUES
		('older','node','node','DEGRADED',$1,'legacy duplicate'),('newer','node','node','OFFLINE',$2,'legacy duplicate')`,
		pricingStamp(at), pricingStamp(at.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	recovered := at.Add(2 * time.Minute)
	if err := store.RecordPricingHealth(ctx, pricing.Health{NodeID: "node", State: "HEALTHY", At: recovered}); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []time.Time{at.Add(90 * time.Second), recovered} {
		if err := store.RecordPricingHealth(ctx, pricing.Health{NodeID: "node", State: "OFFLINE", At: stale}); err != nil {
			t.Fatal(err)
		}
	}
	var total, open, closedAt int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE ended_at IS NULL),
		COUNT(*) FILTER(WHERE ended_at=$1) FROM pricing_operational_incidents WHERE scope_id='node'`, pricingStamp(recovered)).Scan(&total, &open, &closedAt); err != nil || total != 2 || open != 0 || closedAt != 2 {
		t.Fatalf("recovered history = total %d, open %d, exact closed %d: %v", total, open, closedAt, err)
	}
	if err := store.RecordPricingHealth(ctx, pricing.Health{NodeID: "removed-node", State: "OFFLINE", At: recovered.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_operational_incidents WHERE scope_id='removed-node'`).Scan(&total); err != nil || total != 0 {
		t.Fatalf("removed-node history = %d, %v", total, err)
	}
	// A strictly newer unhealthy transition remains observable after recovery.
	if err := store.RecordPricingHealth(ctx, pricing.Health{NodeID: "node", State: "OVERLOADED", At: recovered.Add(time.Nanosecond)}); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_operational_incidents WHERE scope_id='node' AND ended_at IS NULL AND state='OVERLOADED'`).Scan(&open); err != nil || open != 1 {
		t.Fatalf("new transition = %d, %v", open, err)
	}
}
