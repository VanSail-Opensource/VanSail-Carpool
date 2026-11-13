package database

import (
	"context"
	"database/sql"
	"time"
)

// A complete cohort is sealed before its first bounded source deletion. Later
// observations can mark it stale, but cannot rebuild it from a partial history.
func sealPricingValidationTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE pricing_model_validation_daily m SET sealed=true WHERE (m.day,m.model_version,m.metric,m.horizon_seconds) IN (
 SELECT c.day,c.model_version,c.metric,c.horizon_seconds FROM pricing_model_validation_daily c
 WHERE NOT c.sealed AND NOT c.dirty AND NOT c.stale
 AND EXISTS(SELECT 1 FROM pricing_forecast_vintages f WHERE SUBSTR(f.target_from,1,10)=c.day AND f.model_version=c.model_version AND f.metric=c.metric AND f.horizon_seconds=c.horizon_seconds
 AND f.target_until<CASE WHEN f.entity_type='user_node' THEN $1 WHEN f.entity_type='user' THEN $2 ELSE $3 END)
 AND NOT EXISTS(SELECT 1 FROM pricing_forecast_vintages f LEFT JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id
 WHERE SUBSTR(f.target_from,1,10)=c.day AND f.model_version=c.model_version AND f.metric=c.metric AND f.horizon_seconds=c.horizon_seconds
 AND (e.forecast_id IS NULL OR f.target_until>$4 OR e.evaluated_at_ns>c.updated_at_ns))
 ORDER BY c.day,c.model_version,c.metric,c.horizon_seconds LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -30)), pricingStamp(now.AddDate(0, 0, -60)), pricingStamp(now.AddDate(0, 0, -90)), pricingStamp(now))
	return err
}

func sealPricingCalibrationTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	bucket := pricingProbabilityBucketSQL("t.predicted_probability")
	_, err := tx.ExecContext(ctx, `UPDATE pricing_calibration_daily c SET sealed=true WHERE c.kind='purchase' AND (c.day,c.model_version) IN (
 SELECT m.day,m.model_version FROM pricing_calibration_daily m WHERE m.kind='purchase' AND NOT m.sealed
 AND NOT EXISTS(SELECT 1 FROM pricing_calibration_daily d WHERE d.day=m.day AND d.model_version=m.model_version AND d.kind='purchase' AND (d.dirty OR d.stale))
 AND EXISTS(SELECT 1 FROM pricing_offers t WHERE SUBSTR(t.exposed_at,1,10)=m.day AND t.model_version=m.model_version AND t.expires_at<$1)
 AND NOT EXISTS(SELECT 1 FROM pricing_offers t WHERE SUBSTR(t.exposed_at,1,10)=m.day AND t.model_version=m.model_version AND (t.outcome IS NULL OR t.expires_at>$2))
 AND NOT EXISTS(SELECT 1 FROM (SELECT `+bucket+` AS bucket,COUNT(*) AS samples,MAX(GREATEST(t.observed_at,t.expires_at)) AS watermark FROM pricing_offers t
 WHERE SUBSTR(t.exposed_at,1,10)=m.day AND t.model_version=m.model_version AND t.outcome IS NOT NULL AND t.outcome_reason<>'INSUFFICIENT_FUNDS' GROUP BY `+bucket+`) evidence
 WHERE NOT EXISTS(SELECT 1 FROM pricing_calibration_daily d WHERE d.day=m.day AND d.model_version=m.model_version AND d.kind='purchase' AND d.bucket=evidence.bucket AND d.samples=evidence.samples AND d.updated_at>=evidence.watermark))
 GROUP BY m.day,m.model_version ORDER BY m.day,m.model_version LIMIT 1000)`, pricingStamp(pricingOfferRetentionCutoff(now)), pricingStamp(now))
	return err
}

func pricingOfferRetentionCutoff(now time.Time) time.Time {
	return now.UTC().AddDate(0, 0, -90).Truncate(24 * time.Hour)
}
