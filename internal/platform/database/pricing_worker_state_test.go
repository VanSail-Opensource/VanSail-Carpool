package database

import (
	"context"
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
