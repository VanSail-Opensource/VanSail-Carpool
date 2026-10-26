package database

import (
	"context"
	"database/sql"
	"time"
)

// PricingCleanup prunes only bounded, summarized derived data. Financial facts,
// contracts, approved prices and compact administrative audit are never deleted.
func (s *Store) retryAttempt_PricingCleanup(ctx context.Context, now time.Time) (map[string]int64, error) {
	if err := s.SummarizePricingValidation(ctx, now); err != nil {
		return nil, err
	}
	if err := s.SummarizePricingCalibration(ctx, now); err != nil {
		return nil, err
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	counts := map[string]int64{}
	if _, err = tx.ExecContext(ctx, `UPDATE pricing_discount_spend SET status='RELEASED' WHERE status='RESERVED' AND reference_id IN (SELECT 'renewal:'||id FROM pricing_renewal_quotes WHERE state IN ('CANCELLED','EXPIRED') OR valid_until<=? AND state<>'SETTLED')`, pricingStamp(now)); err != nil {
		return nil, err
	}
	queries := []struct {
		name, query string
		at          any
	}{
		{"compute", `DELETE FROM pricing_features WHERE feature_key IN (SELECT f.feature_key FROM pricing_features f WHERE f.entity_type IN ('compute_input','compute_result') AND f.watermark<? AND NOT EXISTS(SELECT 1 FROM outbox_jobs j WHERE j.kind='pricing_compute' AND json_extract(j.payload,'$.inputId')='compute-input:'||f.entity_id) LIMIT 1000)`, pricingStamp(now.Add(-24 * time.Hour))},
		{"forecasts_user_node", `DELETE FROM pricing_forecast_values WHERE forecast_id IN (SELECT f.forecast_id FROM pricing_forecast_vintages f WHERE f.entity_type='user_node' AND f.target_until<? AND EXISTS(SELECT 1 FROM pricing_model_validation_daily m WHERE m.day=SUBSTR(f.target_from,1,10) AND m.model_version=f.model_version AND m.metric=f.metric AND m.horizon_seconds=f.horizon_seconds) LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -30))},
		{"forecasts_user", `DELETE FROM pricing_forecast_values WHERE forecast_id IN (SELECT f.forecast_id FROM pricing_forecast_vintages f WHERE f.entity_type='user' AND f.target_until<? AND EXISTS(SELECT 1 FROM pricing_model_validation_daily m WHERE m.day=SUBSTR(f.target_from,1,10) AND m.model_version=f.model_version AND m.metric=f.metric AND m.horizon_seconds=f.horizon_seconds) LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -60))},
		{"forecasts", `DELETE FROM pricing_forecast_values WHERE forecast_id IN (SELECT f.forecast_id FROM pricing_forecast_vintages f WHERE f.target_until<? AND EXISTS(SELECT 1 FROM pricing_model_validation_daily m WHERE m.day=SUBSTR(f.target_from,1,10) AND m.model_version=f.model_version AND m.metric=f.metric AND m.horizon_seconds=f.horizon_seconds) LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -90))},
		{"batches", `DELETE FROM pricing_forecast_batches WHERE batch_key IN (SELECT b.batch_key FROM pricing_forecast_batches b WHERE generated_at<? AND NOT EXISTS(SELECT 1 FROM pricing_forecast_values v WHERE v.batch_key=b.batch_key) LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -90))},
		{"features", `DELETE FROM pricing_features WHERE feature_key IN (SELECT f.feature_key FROM pricing_features f WHERE day<? AND NOT EXISTS(SELECT 1 FROM pricing_forecast_values v WHERE v.feature_key=f.feature_key) LIMIT 1000)`, now.AddDate(0, 0, -60).Format("2006-01-02")},
		{"offers", `DELETE FROM pricing_offers WHERE id IN (SELECT id FROM pricing_offers WHERE expires_at<? AND outcome IS NOT NULL AND EXISTS(SELECT 1 FROM pricing_calibration_daily c WHERE c.day=SUBSTR(pricing_offers.exposed_at,1,10) AND c.model_version=pricing_offers.model_version AND c.kind='purchase') LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -60))},
		{"decisions", `DELETE FROM pricing_decisions WHERE id IN (SELECT id FROM pricing_decisions WHERE generated_at<? LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -60))},
		{"incidents", `DELETE FROM pricing_operational_incidents WHERE id IN (SELECT id FROM pricing_operational_incidents WHERE ended_at<? LIMIT 1000)`, pricingStamp(now.AddDate(0, 0, -90))},
		{"calibration", `DELETE FROM pricing_calibration_daily WHERE day<?`, now.AddDate(-1, 0, 0).Format("2006-01-02")},
		{"summaries", `DELETE FROM pricing_model_validation_daily WHERE day<?`, now.AddDate(-1, 0, 0).Format("2006-01-02")},
	}
	for _, q := range queries {
		result, e := tx.ExecContext(ctx, q.query, q.at)
		if e != nil {
			return nil, e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return nil, e
		}
		counts[q.name] = n
	}
	if err = expirePricingOverridesTx(ctx, tx, now); err != nil {
		return nil, err
	}
	return counts, tx.Commit()
}

func expirePricingOverridesTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	// Clearing an expired presentation status is version-guarded. Current policy
	// and health, rather than an obsolete saved price, resolve the next offer.
	if _, err := tx.ExecContext(ctx, `UPDATE pricing_product_settings SET override_status='NONE',version=version+1,updated_at=? WHERE override_status<>'NONE' AND EXISTS(SELECT 1 FROM pricing_admin_overrides o WHERE o.scope_type='product' AND o.scope_id=pricing_product_settings.product_type||':'||pricing_product_settings.product_id AND o.expected_version=pricing_product_settings.version AND (o.expires_at<=? OR o.status='CANCELLED'))`, pricingStamp(now), pricingStamp(now)); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE pricing_admin_overrides SET status='EXPIRED' WHERE expires_at<=? AND status='ACTIVE'`, pricingStamp(now))
	return err
}
