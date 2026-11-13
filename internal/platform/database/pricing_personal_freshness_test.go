package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
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
	_, err = store.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES($1,'user',$2,$3,$4,0,'UNKNOWN','{}','fixture')`, "user:"+user.ID+":"+now.Format(time.DateOnly), user.ID, now.Format(time.DateOnly), pricingStamp(now))
	if err != nil {
		t.Fatal(err)
	}
	forecast, err = store.personalForecast(ctx, user.ID, now)
	if err != nil || forecast.Stale || forecast.RefreshPending || forecast.Status != "MISSING" {
		t.Fatalf("missing forecast: %+v, %v", forecast, err)
	}
}

func TestPersonalFeatureRenewalsRespectExactAsOfWindow(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	user := createTestUser(t, s, 78313)
	now := time.Date(2030, 1, 31, 0, 0, 0, 0, time.UTC)
	from := now.AddDate(0, 0, -30)
	for i, at := range []time.Time{from.Add(-time.Nanosecond), from, now, now.Add(time.Nanosecond)} {
		id := fmt.Sprintf("renewal-%d", i)
		if _, err := s.db.ExecContext(ctx, `INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,created_at) VALUES($1,$2,-1,0,'automatic_renewal',$1,$3)`, id, user.ID, stamp(at)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.pricingUserFeatures(ctx, user.ID, now); !errors.Is(err, ErrPricingComputePending) {
		t.Fatal(err)
	}
	job, err := s.ClaimComputeOutboxJob(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	var payload struct {
		ID string `json:"inputId"`
	}
	if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	task, err := s.PricingComputeInput(ctx, payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := compute.Run(ctx, compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AcceptPricingCompute(ctx, task, out.Items[0], now); err != nil {
		t.Fatal(err)
	}
	if err = s.pricingUserFeatures(ctx, user.ID, now); err != nil {
		t.Fatal(err)
	}
	var renewals int
	if err = s.db.QueryRowContext(ctx, `SELECT (features_json::jsonb->>'renewals30')::integer FROM pricing_features WHERE entity_type='user' AND entity_id=$1`, user.ID).Scan(&renewals); err != nil || renewals != 2 {
		t.Fatal("feature included future or expired renewal evidence", renewals, err)
	}
}

func TestPersonalForecastOldVintageRemainsStale(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 78312)
	ctx := context.Background()
	now := time.Now().UTC()
	day := now.Truncate(24 * time.Hour)
	generated := day.Add(-time.Hour)
	_, err := store.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES($1,'user',$2,$3,$4,10000,'UNKNOWN','{}','old-feature')`, "old-user-feature", user.ID, generated.Format(time.DateOnly), pricingStamp(generated))
	if err != nil {
		t.Fatal(err)
	}
	forecast := pricing.Forecast{ID: "old-vintage", EntityType: "user", EntityID: user.ID, Metric: "traffic_bytes", GeneratedAt: generated, TargetFrom: day.Add(24 * time.Hour), TargetUntil: day.Add(48 * time.Hour), HorizonSeconds: 86400, P10: 1, P50: 2, P90: 3, Mean: 2, ModelVersion: "empirical-user-total-v1", Watermark: generated, SnapshotHash: "old-feature", CoverageBPS: 10000, HealthState: "UNKNOWN", Calibration: "UNVALIDATED"}
	if err = store.SavePricingForecast(ctx, forecast); err != nil {
		t.Fatal(err)
	}
	// A later feature with no complete total observations must not hide a
	// still-future vintage from the previous day.
	_, err = store.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES($1,'user',$2,$3,$4,0,'UNKNOWN','{}','empty-feature')`, "empty-user-feature", user.ID, day.Format(time.DateOnly), pricingStamp(now))
	if err != nil {
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
	if err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1`, PricingComputeJob).Scan(&queued); err != nil || queued != 0 {
		t.Fatalf("queued calculations while busy: %d, %v", queued, err)
	}
}
