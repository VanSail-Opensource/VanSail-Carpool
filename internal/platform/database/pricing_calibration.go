package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SummarizePricingCalibration preserves original probability forecasts as
// probability buckets, not a fabricated continuous purchase/revenue curve.
func (s *Store) retryAttempt_SummarizePricingCalibration(ctx context.Context, now time.Time) error {
	tx, err := s.beginPricingSummary(ctx, "calibration")
	if err != nil || tx == nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, kind := range []string{"purchase", "renewal"} {
		dateExpr, modelExpr, probExpr, outcomeExpr, observedExpr, source := `t.exposed_at`, `t.model_version`, `t.predicted_probability`, `t.outcome`, `GREATEST(t.observed_at,t.expires_at)`, `pricing_offers t WHERE t.outcome_reason<>'INSUFFICIENT_FUNDS' AND t.outcome IS NOT NULL AND t.expires_at<=$1`
		identities := `pricing_offers t WHERE t.outcome IS NOT NULL AND t.expires_at<=$1`
		if kind == "renewal" {
			dateExpr, modelExpr, probExpr, outcomeExpr, observedExpr, source = `t.generated_at`, `t.model_version`, `t.probability`, `CASE WHEN t.state='SETTLED' THEN 1 ELSE 0 END`, `t.valid_until`, `pricing_renewal_quotes t WHERE t.outcome_reason<>'INSUFFICIENT_FUNDS' AND t.state<>'CANCELLED' AND t.authorized_at IS NOT NULL AND t.valid_until<=$1`
			identities = `pricing_renewal_quotes t WHERE t.authorized_at IS NOT NULL AND t.valid_until<=$1`
		}
		unsealed := ` AND NOT EXISTS(SELECT 1 FROM pricing_calibration_daily sealed WHERE sealed.day=SUBSTR(` + dateExpr + `,1,10) AND sealed.model_version=` + modelExpr + ` AND sealed.kind='` + kind + `' AND sealed.sealed)`
		source, identities = source+unsealed, identities+unsealed
		bucket := pricingProbabilityBucketSQL(probExpr)
		var day string
		evidence := `SELECT SUBSTR(` + dateExpr + `,1,10) AS day,` + modelExpr + ` AS model_version,` + bucket + ` AS bucket,COUNT(*) AS samples,MAX(` + observedExpr + `) AS watermark FROM ` + source + ` AND ` + dateExpr + `>=$2 GROUP BY SUBSTR(` + dateExpr + `,1,10),` + modelExpr + `,` + bucket
		query := `SELECT candidate.day FROM (SELECT e.day FROM (` + evidence + `) e WHERE NOT EXISTS(SELECT 1 FROM pricing_calibration_daily c WHERE c.day=e.day AND c.model_version=e.model_version AND c.kind=$3 AND c.bucket=e.bucket AND NOT c.dirty AND c.samples=e.samples AND c.updated_at>=e.watermark) UNION SELECT SUBSTR(` + dateExpr + `,1,10) FROM ` + identities + ` AND ` + dateExpr + `>=$2 AND NOT EXISTS(SELECT 1 FROM pricing_calibration_daily c WHERE c.day=SUBSTR(` + dateExpr + `,1,10) AND c.model_version=` + modelExpr + ` AND c.kind=$3) UNION SELECT c.day FROM pricing_calibration_daily c WHERE c.kind=$3 AND c.dirty AND NOT c.sealed) candidate ORDER BY candidate.day LIMIT 1`
		err := tx.QueryRowContext(ctx, query, pricingStamp(now), pricingStamp(now.AddDate(0, 0, -90)), kind).Scan(&day)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		query = `SELECT ` + modelExpr + `,` + bucket + `,COUNT(*),SUM(` + probExpr + `),SUM(` + outcomeExpr + `),SUM((` + probExpr + `-(` + outcomeExpr + `))*(` + probExpr + `-(` + outcomeExpr + `))),MAX(` + observedExpr + `) FROM ` + source + ` AND SUBSTR(` + dateExpr + `,1,10)=$2 GROUP BY ` + modelExpr + `,` + bucket
		rows, err := tx.QueryContext(ctx, query, pricingStamp(now), day)
		if err != nil {
			return err
		}
		type summary struct {
			model, watermark          string
			bucket, samples, observed int
			probability, brier        float64
		}
		values := []summary{}
		for rows.Next() {
			var v summary
			if err = rows.Scan(&v.model, &v.bucket, &v.samples, &v.probability, &v.observed, &v.brier, &v.watermark); err != nil {
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
		present := map[string]bool{}
		for _, v := range values {
			present[v.model] = true
		}
		rows, err = tx.QueryContext(ctx, `SELECT `+modelExpr+`,MAX(`+observedExpr+`) FROM `+identities+` AND SUBSTR(`+dateExpr+`,1,10)=$2 GROUP BY `+modelExpr, pricingStamp(now), day)
		if err != nil {
			return err
		}
		for rows.Next() {
			var v summary
			if err = rows.Scan(&v.model, &v.watermark); err != nil {
				rows.Close()
				return err
			}
			if !present[v.model] {
				values = append(values, v) // A fully excluded cohort has zero observations.
				present[v.model] = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = tx.QueryContext(ctx, `DELETE FROM pricing_calibration_daily WHERE day=$1 AND kind=$2 AND NOT sealed RETURNING model_version`, day, kind)
		if err != nil {
			return err
		}
		for rows.Next() {
			var model string
			if err = rows.Scan(&model); err != nil {
				rows.Close()
				return err
			}
			if !present[model] {
				// An empty existing cohort still needs an identity for later
				// corrections after the initial 90-day selection window.
				values = append(values, summary{model: model, watermark: pricingStamp(now)})
				present[model] = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, v := range values {
			if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_calibration_daily(day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(day,model_version,kind,bucket) DO UPDATE SET samples=excluded.samples,probability_sum=excluded.probability_sum,observed_sum=excluded.observed_sum,brier_sum=excluded.brier_sum,updated_at=excluded.updated_at,dirty=false WHERE NOT pricing_calibration_daily.sealed`, day, v.model, kind, v.bucket, v.samples, v.probability, v.observed, v.brier, v.watermark); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func pricingProbabilityBucketSQL(expression string) string {
	return `CASE WHEN ` + expression + `>=0.9 THEN 9 WHEN ` + expression + `>=0.8 THEN 8 WHEN ` + expression + `>=0.7 THEN 7 WHEN ` + expression + `>=0.6 THEN 6 WHEN ` + expression + `>=0.5 THEN 5 WHEN ` + expression + `>=0.4 THEN 4 WHEN ` + expression + `>=0.3 THEN 3 WHEN ` + expression + `>=0.2 THEN 2 WHEN ` + expression + `>=0.1 THEN 1 ELSE 0 END`
}
