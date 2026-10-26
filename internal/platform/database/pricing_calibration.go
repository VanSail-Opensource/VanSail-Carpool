package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SummarizePricingCalibration preserves original probability forecasts as
// probability buckets, not a fabricated continuous purchase/revenue curve.
func (s *Store) SummarizePricingCalibration(ctx context.Context, now time.Time) error {
	for _, kind := range []string{"purchase", "renewal"} {
		dateExpr, modelExpr, probExpr, outcomeExpr, observedExpr, source := `t.exposed_at`, `t.model_version`, `t.predicted_probability`, `t.outcome`, `t.observed_at`, `pricing_offers t WHERE t.outcome_reason<>'INSUFFICIENT_FUNDS' AND t.outcome IS NOT NULL AND t.expires_at<=?`
		if kind == "renewal" {
			dateExpr, modelExpr, probExpr, outcomeExpr, observedExpr, source = `t.generated_at`, `t.model_version`, `t.probability`, `CASE WHEN t.state='SETTLED' THEN 1 ELSE 0 END`, `t.valid_until`, `pricing_renewal_quotes t WHERE t.outcome_reason<>'INSUFFICIENT_FUNDS' AND t.state<>'CANCELLED' AND t.authorized_at IS NOT NULL AND t.valid_until<=?`
		}
		var day string
		query := `SELECT SUBSTR(` + dateExpr + `,1,10) FROM ` + source + ` AND ` + dateExpr + `>=? AND NOT EXISTS(SELECT 1 FROM pricing_calibration_daily c WHERE c.day=SUBSTR(` + dateExpr + `,1,10) AND c.model_version=` + modelExpr + ` AND c.kind=? AND c.updated_at>=` + observedExpr + `) ORDER BY ` + dateExpr + ` LIMIT 1`
		err := s.db.QueryRowContext(ctx, query, pricingStamp(now), pricingStamp(now.AddDate(0, 0, -90)), kind).Scan(&day)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		bucket := `CASE WHEN ` + probExpr + `>=0.9 THEN 9 WHEN ` + probExpr + `>=0.8 THEN 8 WHEN ` + probExpr + `>=0.7 THEN 7 WHEN ` + probExpr + `>=0.6 THEN 6 WHEN ` + probExpr + `>=0.5 THEN 5 WHEN ` + probExpr + `>=0.4 THEN 4 WHEN ` + probExpr + `>=0.3 THEN 3 WHEN ` + probExpr + `>=0.2 THEN 2 WHEN ` + probExpr + `>=0.1 THEN 1 ELSE 0 END`
		query = `SELECT ` + modelExpr + `,` + bucket + `,COUNT(*),SUM(` + probExpr + `),SUM(` + outcomeExpr + `),SUM((` + probExpr + `-(` + outcomeExpr + `))*(` + probExpr + `-(` + outcomeExpr + `))) FROM ` + source + ` AND SUBSTR(` + dateExpr + `,1,10)=? GROUP BY ` + modelExpr + `,` + bucket
		rows, err := s.db.QueryContext(ctx, query, pricingStamp(now), day)
		if err != nil {
			return err
		}
		type summary struct {
			model                     string
			bucket, samples, observed int
			probability, brier        float64
		}
		values := []summary{}
		for rows.Next() {
			var v summary
			if err = rows.Scan(&v.model, &v.bucket, &v.samples, &v.probability, &v.observed, &v.brier); err != nil {
				rows.Close()
				return err
			}
			values = append(values, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, v := range values {
			if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_calibration_daily(day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(day,model_version,kind,bucket) DO UPDATE SET samples=excluded.samples,probability_sum=excluded.probability_sum,observed_sum=excluded.observed_sum,brier_sum=excluded.brier_sum,updated_at=excluded.updated_at`, day, v.model, kind, v.bucket, v.samples, v.probability, v.observed, v.brier, pricingStamp(now)); err != nil {
				return err
			}
		}
	}
	return nil
}
