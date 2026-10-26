package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// SavePricingForecast never updates a previous vintage, including on retry.
func (s *Store) SavePricingForecast(ctx context.Context, f pricing.Forecast) error {
	if err := validatePricingForecast(f); err != nil {
		return err
	}
	return s.saveNormalizedPricingForecast(ctx, f)
}

func validatePricingForecast(f pricing.Forecast) error {
	for _, v := range []float64{f.P10, f.P50, f.P90, f.Mean} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ErrConflict
		}
	}
	if f.ID == "" || f.GeneratedAt.After(f.TargetFrom) || f.Watermark.After(f.GeneratedAt) || !f.TargetUntil.After(f.TargetFrom) || f.P10 > f.P50 || f.P50 > f.P90 || f.CoverageBPS < 0 || f.CoverageBPS > 10000 || math.IsNaN(f.Mean) || math.IsInf(f.Mean, 0) {
		return ErrConflict
	}
	return nil
}

// EvaluatePricingForecast saves actuals separately and rejects incomplete/early zeroes.
func (s *Store) EvaluatePricingForecast(ctx context.Context, id, status string, actual *float64, observed, now time.Time) error {
	id = pricingForecastID(id)
	var until string
	if err := s.db.QueryRowContext(ctx, `SELECT target_until FROM pricing_forecast_vintages WHERE forecast_id=?`, id).Scan(&until); err != nil {
		return err
	}
	end, err := parseStamp(until)
	if err != nil {
		return err
	}
	if end.After(now) || observed.Before(end) {
		return ErrConflict
	}
	if status != "COMPLETE" && status != "MISSING" && status != "CENSORED" && status != "INCOMPLETE" {
		return ErrConflict
	}
	if (status == "COMPLETE") != (actual != nil) || actual != nil && (math.IsNaN(*actual) || math.IsInf(*actual, 0)) {
		return ErrConflict
	}
	code := map[string]int{"COMPLETE": 0, "MISSING": 1, "CENSORED": 2, "INCOMPLETE": 3}[status]
	_, err = s.db.ExecContext(ctx, `INSERT INTO pricing_forecast_actuals(forecast_id,actual,status_code,observed_until_ns,evaluated_at_ns) VALUES(?,?,?,?,?) ON CONFLICT(forecast_id) DO UPDATE SET actual=excluded.actual,status_code=excluded.status_code,observed_until_ns=excluded.observed_until_ns,evaluated_at_ns=excluded.evaluated_at_ns WHERE pricing_forecast_actuals.observed_until_ns<=excluded.observed_until_ns AND pricing_forecast_actuals.evaluated_at_ns<=excluded.evaluated_at_ns`, id, actual, code, observed.UnixNano(), now.UnixNano())
	return err
}

// PricingFeatureBatch generates only present-day predictions from retained accounting.
// A restart cannot fabricate a past vintage; daily uniqueness suppresses duplicates.
func (s *Store) PricingFeatureBatch(ctx context.Context, now time.Time, cursor string) (string, error) {
	release, slotErr := s.acquirePricingCompute()
	if slotErr != nil {
		return cursor, slotErr
	}
	defer release()
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM xnode_nodes n WHERE enabled=1 AND id>? AND NOT EXISTS(SELECT 1 FROM pricing_features f WHERE f.entity_type='node' AND f.entity_id=n.id AND f.day=?) ORDER BY id LIMIT 16`, cursor, now.UTC().Format("2006-01-02"))
	if err != nil {
		return cursor, err
	}
	nodes := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return cursor, err
		}
		nodes = append(nodes, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return cursor, err
	}
	for _, node := range nodes {
		if err = s.pricingNodeForecast(ctx, node, now); err != nil {
			if errors.Is(err, ErrPricingComputePending) {
				continue
			}
			return cursor, err
		}
	}
	if len(nodes) < 16 {
		return "", nil
	}
	return nodes[len(nodes)-1], nil
}

func (s *Store) pricingNodeForecast(ctx context.Context, node string, now time.Time) error {
	day := now.UTC().Truncate(24 * time.Hour)
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_features WHERE entity_type='node' AND entity_id=? AND day=?)`, node, day.Format("2006-01-02")).Scan(&exists); err != nil || exists == 1 {
		return err
	}
	// Existing raw accounting is the only traffic source. Missing dates are never
	// padded with invented zero usage, including during node outages.
	rows, err := s.db.QueryContext(ctx, `SELECT date,SUM(upload_bytes+download_bytes) FROM xnode_daily_usage WHERE node_id=? AND date>=? AND date<? GROUP BY date ORDER BY date`, node, day.AddDate(0, 0, -30).Format("2006-01-02"), day.Format("2006-01-02"))
	if err != nil {
		return err
	}
	samples := []float64{}
	source := []map[string]any{}
	for rows.Next() {
		var date string
		var bytes float64
		if err = rows.Scan(&date, &bytes); err != nil {
			rows.Close()
			return err
		}
		samples = append(samples, bytes)
		source = append(source, map[string]any{"date": date, "trafficBytes": bytes})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"samples": source, "definition": "raw native upload+download; missing days excluded", "sampleCount": len(samples)})
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	id := "node:" + node + ":" + day.Format("2006-01-02")
	coverage := int64(len(samples)) * 10000 / 30
	health := "TELEMETRY_STALE"
	for _, h := range s.pricingHealth.Snapshot(now) {
		if h.NodeID == node {
			health = h.State
			break
		}
	}
	q, err := s.pricingQuantiles(ctx, samples)
	if err != nil {
		return err
	}
	feature := pricingFeatureSnapshot{ID: id, Kind: "node", Entity: node, Day: day.Format(time.DateOnly), At: now, Coverage: coverage, Health: health, Raw: string(raw), Hash: digest}
	forecasts := []pricing.Forecast{}
	p10, p50, p90, mean := q.P10, q.P50, q.P90, q.Mean
	if health != "HEALTHY" {
		p10 *= .5
		p90 = max(p90*1.5, mean*2)
	}
	for _, lead := range []int{1, 7} {
		if len(samples) == 0 {
			break
		}
		from := day.AddDate(0, 0, lead)
		f := pricing.Forecast{ID: id + ":" + from.Format("2006-01-02"), EntityType: "node", EntityID: node, Metric: "traffic_bytes", GeneratedAt: now, TargetFrom: from, TargetUntil: from.Add(24 * time.Hour), HorizonSeconds: int64(lead) * 86400, P10: p10, P50: p50, P90: p90, Mean: mean, ModelVersion: "empirical-daily-v1", Watermark: now, SnapshotHash: digest, CoverageBPS: coverage, HealthState: health, Calibration: "UNVALIDATED"}
		forecastHash := sha256.Sum256([]byte(f.ID))
		f.ID = hex.EncodeToString(forecastHash[:16])
		if len(samples) < 7 {
			f.Calibration = "PRIOR_ONLY"
		}
		forecasts = append(forecasts, f)
	}
	return s.savePricingFeatureForecasts(ctx, feature, forecasts)
}

// RegisterPricingModels saves reproducible algorithm definitions once.
func (s *Store) RegisterPricingModels(ctx context.Context, now time.Time) error {
	for _, m := range []struct{ id, kind, definition string }{{"empirical-user-total-v1", "traffic", `{"lookbackDays":30,"quantiles":[0.1,0.5,0.9],"missing":"exclude days lacking any observed node","unit":"bytes/day","horizon":"UTC complete accounting days 1..5","calibration":"original future evaluations required"}`}, {"purchase-prior-v1", "purchase", `{"alpha":1,"beta":1,"probability":0.5}`}, {"empirical-receipts-v1", "revenue", `{"quantiles":[0.1,0.5,0.9],"lookbackDays":30,"units":"TXB minor units/day before refunds"}`}} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_model_versions(id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json) VALUES(?,?,?,'PRIOR_ONLY',?,'{}',?,'{}') ON CONFLICT(id) DO NOTHING`, m.id, m.kind, pricingStamp(now), m.definition, pricingStamp(now)); err != nil {
			return err
		}
	}
	for _, m := range []struct{ id, kind, parameters, features string }{{"empirical-daily-v1", "traffic", `{"quantiles":[0.1,0.5,0.9],"lookbackDays":30,"missing":"excluded","healthInflation":1.5}`, `{"source":"xnode_daily_usage","unit":"bytes/day","timezone":"UTC"}`}, {"renewal-prior-v1", "renewal", `{"alpha":1,"beta":1,"elasticityValidated":false}`, `{"source":"system renewal basis snapshots","unit":"probability"}`}} {
		_, err := s.db.ExecContext(ctx, `INSERT INTO pricing_model_versions(id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json) VALUES(?,?,?,'PRIOR_ONLY',?,?,?,'{}') ON CONFLICT(id) DO NOTHING`, m.id, m.kind, pricingStamp(now), m.parameters, m.features, pricingStamp(now))
		if err != nil {
			return err
		}
	}
	return nil
}
