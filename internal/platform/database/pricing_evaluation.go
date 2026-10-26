package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// ReconcilePricingForecasts revisits recent actuals for delayed reports, without
// changing original predictions or interpreting missing observations as zero.
func (s *Store) ReconcilePricingForecasts(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT f.forecast_id,f.entity_type,f.entity_id,f.metric,f.target_from,f.target_until FROM pricing_forecast_vintages f LEFT JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE f.target_until<=? AND (e.forecast_id IS NULL OR e.evaluated_at_ns<? AND (e.status<>'COMPLETE' OR f.target_until>?)) ORDER BY CASE WHEN e.forecast_id IS NULL THEN 0 ELSE 1 END,f.target_until DESC,f.forecast_id LIMIT 64`, pricingStamp(now.Add(-48*time.Hour)), now.Add(-24*time.Hour).UnixNano(), pricingStamp(now.AddDate(0, 0, -7)))
	if err != nil {
		return err
	}
	type pending struct{ id, kind, entity, metric, from, until string }
	items := []pending{}
	for rows.Next() {
		var p pending
		if err = rows.Scan(&p.id, &p.kind, &p.entity, &p.metric, &p.from, &p.until); err != nil {
			rows.Close()
			return err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range items {
		from, e := parseStamp(p.from)
		if e != nil {
			return e
		}
		until, e := parseStamp(p.until)
		if e != nil {
			return e
		}
		actual, status, e := s.pricingActual(ctx, p.kind, p.entity, p.metric, from, until)
		if e != nil {
			return e
		}
		if err = s.EvaluatePricingForecast(ctx, p.id, status, actual, until, now); err != nil {
			return err
		}
	}
	return nil
}

// SummarizePricingValidation consumes one full target day per pass. It refuses
// partial statistics if an unexpectedly large day exceeds the bounded budget.
func (s *Store) SummarizePricingValidation(ctx context.Context, now time.Time) error {
	var day string
	err := s.db.QueryRowContext(ctx, `SELECT SUBSTR(f.target_from,1,10) FROM pricing_forecast_vintages f JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE f.target_until<=? AND NOT EXISTS(SELECT 1 FROM pricing_model_validation_daily m WHERE m.day=SUBSTR(f.target_from,1,10) AND m.model_version=f.model_version AND m.metric=f.metric AND m.horizon_seconds=f.horizon_seconds AND m.updated_at_ns>=e.evaluated_at_ns) ORDER BY f.target_from LIMIT 1`, pricingStamp(now)).Scan(&day)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT f.model_version,f.metric,f.horizon_seconds,f.predicted_p50,f.predicted_p10,f.predicted_p90,e.actual,e.status FROM pricing_forecast_vintages f JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE SUBSTR(f.target_from,1,10)=? ORDER BY f.forecast_id LIMIT 20001`, day)
	if err != nil {
		return err
	}
	type key struct {
		model, metric string
		horizon       int64
	}
	groups := map[key][]pricing.Evaluation{}
	count := 0
	for rows.Next() {
		count++
		if count > 20000 {
			rows.Close()
			return pricing.StoragePressure
		}
		var k key
		var e pricing.Evaluation
		if err = rows.Scan(&k.model, &k.metric, &k.horizon, &e.Predicted, &e.P10, &e.P90, &e.Actual, &e.Status); err != nil {
			rows.Close()
			return err
		}
		groups[k] = append(groups[k], e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for k, values := range groups {
		v := pricing.Evaluate(values)
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_model_validation_daily(day,model_version,metric,horizon_seconds,samples,metrics_json,updated_at_ns) VALUES(?,?,?,?,?,?,?) ON CONFLICT(day,model_version,metric,horizon_seconds) DO UPDATE SET samples=excluded.samples,metrics_json=excluded.metrics_json,updated_at_ns=excluded.updated_at_ns`, day, k.model, k.metric, k.horizon, v.Samples, string(raw), now.UnixNano()); err != nil {
			return err
		}
	}
	return nil
}
