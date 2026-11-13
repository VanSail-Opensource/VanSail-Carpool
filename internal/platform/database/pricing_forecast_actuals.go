package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
)

// EvaluatePricingForecast saves actuals separately and rejects incomplete/early zeroes.
func (s *Store) EvaluatePricingForecast(ctx context.Context, id, status string, actual *float64, observed, now time.Time) error {
	id = pricingForecastID(id)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockPricingValidationSourceTx(ctx, tx); err != nil {
		return err
	}
	var until string
	if err = tx.QueryRowContext(ctx, `SELECT target_until FROM pricing_forecast_vintages WHERE forecast_id=$1`, id).Scan(&until); err != nil {
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
	var previousActual sql.NullFloat64
	var previousStatus int
	var previousObserved int64
	err = tx.QueryRowContext(ctx, `SELECT actual,status_code,observed_until_ns FROM pricing_forecast_actuals WHERE forecast_id=$1 FOR UPDATE`, id).Scan(&previousActual, &previousStatus, &previousObserved)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	material := errors.Is(err, sql.ErrNoRows) || previousStatus != code || previousObserved != observed.UnixNano() || previousActual.Valid != (actual != nil)
	if actual != nil && previousActual.Valid && previousActual.Float64 != *actual {
		material = true
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO pricing_forecast_actuals(forecast_id,actual,status_code,observed_until_ns,evaluated_at_ns) VALUES($1,$2,$3,$4,$5) ON CONFLICT(forecast_id) DO UPDATE SET actual=excluded.actual,status_code=excluded.status_code,observed_until_ns=excluded.observed_until_ns,evaluated_at_ns=excluded.evaluated_at_ns WHERE pricing_forecast_actuals.observed_until_ns<=excluded.observed_until_ns AND pricing_forecast_actuals.evaluated_at_ns<=excluded.evaluated_at_ns AND (pricing_forecast_actuals.actual,pricing_forecast_actuals.status_code,pricing_forecast_actuals.observed_until_ns,pricing_forecast_actuals.evaluated_at_ns) IS DISTINCT FROM (excluded.actual,excluded.status_code,excluded.observed_until_ns,excluded.evaluated_at_ns)`, id, actual, code, observed.UnixNano(), now.UnixNano())
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed > 0 {
		if _, err = tx.ExecContext(ctx, `UPDATE pricing_model_validation_daily m SET dirty=NOT m.sealed,stale=m.stale OR m.sealed FROM pricing_forecast_vintages f WHERE f.forecast_id=$1 AND m.day=SUBSTR(f.target_from,1,10) AND m.model_version=f.model_version AND m.metric=f.metric AND m.horizon_seconds=f.horizon_seconds AND (NOT m.sealed AND NOT m.dirty OR m.sealed AND $2 AND NOT m.stale)`, id, material); err != nil {
			return err
		}
	}
	return tx.Commit()
}
