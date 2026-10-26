package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingForecastFixture(t *testing.T, s *Store, id string, now time.Time) pricing.Forecast {
	t.Helper()
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,'node','node',?,?,10000,'HEALTHY','{}',?)`, id, now.Format("2006-01-02"), pricingStamp(now), id); err != nil {
		t.Fatal(err)
	}
	f := pricing.Forecast{ID: id, EntityType: "node", EntityID: "node", Metric: "traffic_bytes", GeneratedAt: now, TargetFrom: now.Add(24 * time.Hour), TargetUntil: now.Add(48 * time.Hour), HorizonSeconds: 86400, P10: 40, P50: 60, P90: 80, Mean: 60, ModelVersion: "original-v1", Watermark: now, SnapshotHash: id, CoverageBPS: 10000, HealthState: "HEALTHY", Calibration: "UNVALIDATED"}
	if err := s.SavePricingForecast(ctx, f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPricingOriginalForecastLateActualAndHorizonIsolation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	f := pricingForecastFixture(t, s, "original", now)
	changed := f
	changed.P50, changed.P90 = 65, 90
	if err := s.SavePricingForecast(ctx, changed); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE pricing_forecast_values SET predicted_p50=65 WHERE forecast_id=?`, pricingForecastID(f.ID)); err == nil {
		t.Fatal("original vintage was mutable")
	}
	if err := s.EvaluatePricingForecast(ctx, f.ID, "MISSING", nil, f.TargetUntil, f.TargetUntil); err != nil {
		t.Fatal(err)
	}
	filter := PricingHistoryFilter{EntityType: "node", EntityID: "node", Metric: "traffic_bytes", ModelVersion: "original-v1", Horizon: 86400}
	rows, metrics, err := s.PricingHistory(ctx, filter, f.TargetUntil)
	if err != nil || len(rows) != 1 || rows[0].P50 != 60 || rows[0].Actual != nil || metrics.Samples != 0 || metrics.Excluded != 1 {
		t.Fatal(rows, metrics, err)
	}
	actual := 70.0
	if err = s.EvaluatePricingForecast(ctx, f.ID, "COMPLETE", &actual, f.TargetUntil, f.TargetUntil.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	s.pricingCache.Clear()
	rows, metrics, err = s.PricingHistory(ctx, filter, f.TargetUntil.Add(time.Hour))
	if err != nil || rows[0].P50 != 60 || rows[0].Actual == nil || *rows[0].Actual != 70 || metrics.Bias != -10 {
		t.Fatal(rows, metrics, err)
	}
	other := f
	other.ID = "seven-day"
	other.GeneratedAt = now.Add(-6 * 24 * time.Hour)
	other.Watermark = other.GeneratedAt
	other.HorizonSeconds = 604800
	other.SnapshotHash = "seven-input"
	if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES('seven-input','node','node',?,?,10000,'HEALTHY','{}','seven-input')`, other.GeneratedAt.Format("2006-01-02"), pricingStamp(other.Watermark)); err != nil {
		t.Fatal(err)
	}
	if err = s.SavePricingForecast(ctx, other); err != nil {
		t.Fatal(err)
	}
	s.pricingCache.Clear()
	rows, _, err = s.PricingHistory(ctx, filter, f.TargetUntil)
	if err != nil || len(rows) != 1 || rows[0].HorizonSeconds != 86400 {
		t.Fatal("horizons mixed", rows, err)
	}
	filter.Horizon = 604800
	rows, _, err = s.PricingHistory(ctx, filter, f.TargetUntil)
	if err != nil || len(rows) != 1 || rows[0].HorizonSeconds != 604800 {
		t.Fatal(rows, err)
	}
	if err = s.SummarizePricingValidation(ctx, f.TargetUntil.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestPricingRelationshipExportScope(t *testing.T) {
	d := pricing.Datasets()[0]
	for _, candidate := range pricing.Datasets() {
		if candidate.Name == "forecasts/historical_predictions" {
			d = candidate
		}
	}
	query, args := pricingDatasetQuery(d, PricingExportScope{IncludeUsers: true, Users: []string{"42"}, Nodes: []string{"node"}})
	s := newTestStore(t)
	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		t.Fatal("relationship scope SQL", query, err)
	}
	rows.Close()
}
