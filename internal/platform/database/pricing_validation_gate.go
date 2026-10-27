package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// pricingForecastValidatedTx evaluates original one-day vintages, not training
// residuals, retrospective predictions or mixed seven-day horizons.
func pricingForecastValidatedTx(ctx context.Context, tx *sql.Tx, node string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT predicted_p50,predicted_p10,predicted_p90,actual,status FROM (SELECT f.predicted_p50,f.predicted_p10,f.predicted_p90,e.actual,e.status,f.target_from,ROW_NUMBER() OVER(PARTITION BY f.target_from ORDER BY f.generated_at DESC) AS ordinal FROM pricing_forecast_vintages f JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE f.entity_type='node' AND f.entity_id=? AND f.metric='traffic_bytes' AND f.model_version='empirical-daily-v1' AND f.horizon_seconds=86400 AND f.data_coverage_bps>=8000) v WHERE ordinal=1 ORDER BY target_from DESC LIMIT 90`, node)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	values := []pricing.Evaluation{}
	for rows.Next() {
		var e pricing.Evaluation
		if err = rows.Scan(&e.Predicted, &e.P10, &e.P90, &e.Actual, &e.Status); err != nil {
			return false, err
		}
		values = append(values, e)
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	v := pricing.Evaluate(values)
	return v.Samples >= 30 && v.WAPE != nil && *v.WAPE <= .35 && v.Coverage >= .65 && v.Coverage <= .95, nil
}
