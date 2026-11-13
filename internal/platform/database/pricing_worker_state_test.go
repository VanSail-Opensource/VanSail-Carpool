package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestPricingWorkerWatermarkDoesNotRegressOnDelayedObservation(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	start := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	for _, after := range []time.Duration{0, 10 * time.Minute, 9 * time.Minute, 10 * time.Minute, 10*time.Minute + 30*time.Second} {
		if err := store.ObservePricingWorker(ctx, start.Add(after)); err != nil {
			t.Fatal(err)
		}
	}
	var watermark string
	if err := store.DB().QueryRowContext(ctx, `SELECT observed_at FROM pricing_worker_watermark WHERE singleton=1`).Scan(&watermark); err != nil || watermark != pricingStamp(start.Add(10*time.Minute+30*time.Second)) {
		t.Fatalf("latest watermark = %q, %v", watermark, err)
	}
	var gaps, original int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE started_at=$1 AND ended_at=$2)
		FROM pricing_operational_incidents WHERE scope_type='global' AND scope_id='monitoring'`,
		pricingStamp(start), pricingStamp(start.Add(10*time.Minute))).Scan(&gaps, &original); err != nil || gaps != 1 || original != 1 {
		t.Fatalf("monitoring gaps = %d, original %d, %v; delayed observation invented a gap", gaps, original, err)
	}
}

func TestPricingOfferClosureDoesNotOverwriteConcurrentPurchase(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json) VALUES('closing','member','combo','combo',100,100,$1,$2,'fixture',0.5,'{}')`, pricingStamp(now.Add(-time.Hour)), pricingStamp(now)); err != nil {
		t.Fatal(err)
	}
	purchase, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer purchase.Rollback()
	if _, err = purchase.ExecContext(ctx, `UPDATE pricing_offers SET outcome=1,observed_at=$1,purchase_id='paid' WHERE id='closing'`, pricingStamp(now.Add(-time.Nanosecond))); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.ClosePricingOffers(ctx, now) }()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'UPDATE pricing_offers SET outcome=0')>0)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("offer closer did not wait for the concurrent purchase")
		}
	}
	if err = purchase.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	var outcome int
	var observed string
	if err = s.db.QueryRowContext(ctx, `SELECT outcome,observed_at FROM pricing_offers WHERE id='closing'`).Scan(&outcome, &observed); err != nil || outcome != 1 || observed != pricingStamp(now.Add(-time.Nanosecond)) {
		t.Fatal("expiry worker replaced a committed purchase with nonpurchase", outcome, observed, err)
	}
}
