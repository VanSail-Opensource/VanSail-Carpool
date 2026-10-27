package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingHistoryFilter fixes entity, horizon and model so unrelated vintages are
// never silently joined into one historical prediction line.
type PricingHistoryFilter struct {
	EntityType, EntityID, Metric, ModelVersion string
	Horizon                                    int64
	From, Until                                time.Time
}

type pricingHistoryResult struct {
	Items   []pricing.Forecast
	Metrics pricing.Validation
}

func (s *Store) PricingHistory(ctx context.Context, filter PricingHistoryFilter, now time.Time) ([]pricing.Forecast, pricing.Validation, error) {
	key, _ := json.Marshal(filter)
	r, err := pricingCached(ctx, s, "history:"+string(key), time.Minute, func() (pricingHistoryResult, error) {
		items, metrics, e := s.pricingHistory(ctx, filter, now)
		return pricingHistoryResult{items, metrics}, e
	})
	return r.Items, r.Metrics, err
}
func (s *Store) pricingHistory(ctx context.Context, filter PricingHistoryFilter, now time.Time) ([]pricing.Forecast, pricing.Validation, error) {
	if filter.EntityID == "" || len(filter.EntityID) > 256 || (filter.EntityType != "node" && filter.EntityType != "user" && filter.EntityType != "user_node" && filter.EntityType != "product") || (filter.Horizon != 604800 && (filter.Horizon < 86400 || filter.Horizon > 432000 || filter.Horizon%86400 != 0)) {
		return nil, pricing.Validation{}, ErrConflict
	}
	conditions := []string{"f.entity_type=?", "f.entity_id=?", "f.metric=?", "f.horizon_seconds>?", "f.horizon_seconds<=?"}
	args := []any{filter.EntityType, filter.EntityID, filter.Metric, filter.Horizon - 3600, filter.Horizon}
	if filter.ModelVersion != "" {
		conditions = append(conditions, "f.model_version=?")
		args = append(args, filter.ModelVersion)
	}
	if !filter.From.IsZero() {
		conditions = append(conditions, "f.target_from>=?")
		args = append(args, pricingStamp(filter.From))
	}
	if !filter.Until.IsZero() {
		conditions = append(conditions, "f.target_from<?")
		args = append(args, pricingStamp(filter.Until))
	}
	rows, err := s.db.QueryContext(ctx, `SELECT f.forecast_id,f.entity_type,f.entity_id,f.metric,f.generated_at,f.target_from,f.target_until,f.horizon_seconds,f.predicted_p10,f.predicted_p50,f.predicted_p90,f.predicted_mean,f.model_version,f.feature_watermark,f.feature_snapshot_hash,f.data_coverage_bps,f.health_state,f.calibration_status,e.actual,e.status FROM pricing_forecast_vintages f LEFT JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE `+strings.Join(conditions, " AND ")+` ORDER BY f.target_from DESC,f.generated_at DESC LIMIT 500`, args...)
	if err != nil {
		return nil, pricing.Validation{}, err
	}
	defer rows.Close()
	out := []pricing.Forecast{}
	evaluations := []pricing.Evaluation{}
	seen := map[string]bool{}
	for rows.Next() {
		var f pricing.Forecast
		var generated, from, until, watermark string
		var status *string
		if err = rows.Scan(&f.ID, &f.EntityType, &f.EntityID, &f.Metric, &generated, &from, &until, &f.HorizonSeconds, &f.P10, &f.P50, &f.P90, &f.Mean, &f.ModelVersion, &watermark, &f.SnapshotHash, &f.CoverageBPS, &f.HealthState, &f.Calibration, &f.Actual, &status); err != nil {
			return nil, pricing.Validation{}, err
		}
		for _, v := range []struct {
			s string
			t *time.Time
		}{{generated, &f.GeneratedAt}, {from, &f.TargetFrom}, {until, &f.TargetUntil}, {watermark, &f.Watermark}} {
			*v.t, err = parseStamp(v.s)
			if err != nil {
				return nil, pricing.Validation{}, err
			}
		}
		key := from + ":" + f.ModelVersion
		if seen[key] {
			continue
		}
		seen[key] = true
		f.Status = "PENDING"
		if !now.Before(f.TargetUntil) {
			f.Status = "INCOMPLETE"
		}
		if status != nil {
			f.Status = *status
		}
		out = append(out, f)
		evaluations = append(evaluations, pricing.Evaluation{Predicted: f.P50, P10: f.P10, P90: f.P90, Actual: f.Actual, Status: f.Status})
	}
	if err = rows.Err(); err != nil {
		return nil, pricing.Validation{}, err
	}
	return out, pricing.Evaluate(evaluations), nil
}
