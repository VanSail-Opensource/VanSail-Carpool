package database

import (
	"context"
	"database/sql"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Repeated metadata and feature hashes are normalized so the retained daily
// relationship vintages fit the measured storage budget. The public view retains
// the full original forecast contract and every input remains immutable.
func (s *Store) retryAttempt_saveNormalizedPricingForecast(ctx context.Context, f pricing.Forecast) error {
	f.ID = pricingForecastID(f.ID)
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = savePricingForecastTx(ctx, tx, f); err != nil {
		return err
	}
	return tx.Commit()
}

func savePricingForecastTx(ctx context.Context, tx *sql.Tx, f pricing.Forecast) error {
	f.ID = pricingForecastID(f.ID)
	var err error
	if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_forecast_entities(entity_type,entity_id) VALUES(?,?) ON CONFLICT(entity_type,entity_id) DO NOTHING`, f.EntityType, f.EntityID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_forecast_batches(metric,generated_at,target_from,target_until,horizon_seconds,model_version,health_state,calibration_status) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(metric,generated_at,target_from,target_until,horizon_seconds,model_version,health_state,calibration_status) DO NOTHING`, f.Metric, pricingStamp(f.GeneratedAt), pricingStamp(f.TargetFrom), pricingStamp(f.TargetUntil), f.HorizonSeconds, f.ModelVersion, f.HealthState, f.Calibration); err != nil {
		return err
	}
	var entity, batch, feature int64
	if err = tx.QueryRowContext(ctx, `SELECT entity_key FROM pricing_forecast_entities WHERE entity_type=? AND entity_id=?`, f.EntityType, f.EntityID).Scan(&entity); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT batch_key FROM pricing_forecast_batches WHERE metric=? AND generated_at=? AND target_from=? AND target_until=? AND horizon_seconds=? AND model_version=? AND health_state=? AND calibration_status=?`, f.Metric, pricingStamp(f.GeneratedAt), pricingStamp(f.TargetFrom), pricingStamp(f.TargetUntil), f.HorizonSeconds, f.ModelVersion, f.HealthState, f.Calibration).Scan(&batch); err != nil {
		return err
	}
	featureType, featureID := f.EntityType, f.EntityID
	if featureType == "user_node" {
		featureType = "user"
		featureID = strings.SplitN(featureID, ":", 2)[0]
	}
	if err = tx.QueryRowContext(ctx, `SELECT feature_key FROM pricing_features WHERE snapshot_hash=? AND watermark=? AND entity_type=? AND entity_id=? ORDER BY feature_key LIMIT 1`, f.SnapshotHash, pricingStamp(f.Watermark), featureType, featureID).Scan(&feature); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_forecast_values(forecast_id,entity_key,batch_key,feature_key,predicted_p10,predicted_p50,predicted_p90,predicted_mean,data_coverage_bps) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, f.ID, entity, batch, feature, f.P10, f.P50, f.P90, f.Mean, f.CoverageBPS); err != nil {
		return err
	}
	var originalEntity, originalBatch int64
	if err = tx.QueryRowContext(ctx, `SELECT entity_key,batch_key FROM pricing_forecast_values WHERE forecast_id=?`, f.ID).Scan(&originalEntity, &originalBatch); err != nil {
		return err
	}
	if originalEntity != entity || originalBatch != batch {
		return ErrConflict
	}
	return nil
}
