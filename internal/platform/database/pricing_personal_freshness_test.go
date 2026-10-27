package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPersonalForecastMissingIsNotStale(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 78310)
	now := time.Now().UTC()
	ctx := context.Background()

	forecast, err := store.PersonalForecast(ctx, user.ID, now)
	if err != nil || forecast.Stale || !forecast.RefreshPending || forecast.Status != "PENDING" {
		t.Fatalf("pending forecast: %+v, %v", forecast, err)
	}

	// A completed feature without complete observed days cannot produce a
	// forecast; it must remain missing, not claim an old prediction exists.
	_, err = store.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,'user',?,?,?,0,'UNKNOWN','{}','fixture')`, "user:"+user.ID+":"+now.Format(time.DateOnly), user.ID, now.Format(time.DateOnly), pricingStamp(now))
	if err != nil {
		t.Fatal(err)
	}
	forecast, err = store.personalForecast(ctx, user.ID, now)
	if err != nil || forecast.Stale || forecast.RefreshPending || forecast.Status != "MISSING" {
		t.Fatalf("missing forecast: %+v, %v", forecast, err)
	}
}

func TestPersonalForecastOldVintageRemainsStale(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 78312)
	ctx := context.Background()
	now := time.Now().UTC()
	day := now.Truncate(24 * time.Hour)
	generated := day.Add(-time.Hour)
	_, err := store.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,'user',?,?,?,10000,'UNKNOWN','{}','old-feature')`, "old-user-feature", user.ID, generated.Format(time.DateOnly), pricingStamp(generated))
	if err != nil {
		t.Fatal(err)
	}
	forecast := pricing.Forecast{ID: "old-vintage", EntityType: "user", EntityID: user.ID, Metric: "traffic_bytes", GeneratedAt: generated, TargetFrom: day.Add(24 * time.Hour), TargetUntil: day.Add(48 * time.Hour), HorizonSeconds: 86400, P10: 1, P50: 2, P90: 3, Mean: 2, ModelVersion: "empirical-user-total-v1", Watermark: generated, SnapshotHash: "old-feature", CoverageBPS: 10000, HealthState: "UNKNOWN", Calibration: "UNVALIDATED"}
	if err = store.SavePricingForecast(ctx, forecast); err != nil {
		t.Fatal(err)
	}
	out, err := store.personalForecast(ctx, user.ID, now)
	if err != nil || !out.Stale || out.Status != "STALE" {
		t.Fatalf("old forecast: %+v, %v", out, err)
	}
}

func TestPersonalForecastDefersWhenAnalyticsSlotBusy(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 78311)
	ctx := context.Background()
	release, err := store.acquirePricingCompute()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	forecast, err := store.personalForecast(ctx, user.ID, time.Now().UTC())
	if err != nil || !forecast.RefreshPending || forecast.Stale {
		t.Fatalf("deferred forecast: %+v, %v", forecast, err)
	}
	var queued int
	if err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=?`, PricingComputeJob).Scan(&queued); err != nil || queued != 0 {
		t.Fatalf("queued calculations while busy: %d, %v", queued, err)
	}
}
