package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

type pricingFeatureSnapshot struct {
	ID, Kind, Entity, Day, Health, Raw, Hash string
	At                                       time.Time
	Coverage                                 int64
}

// A daily feature and its forecasts are one publication. A crash cannot leave a
// feature that suppresses regeneration of missing forecast days on restart.
func (s *Store) retryAttempt_savePricingFeatureForecasts(ctx context.Context, f pricingFeatureSnapshot, forecasts []pricing.Forecast) error {

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockPricingValidationSourceTx(ctx, tx); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(entity_type,entity_id,day) DO NOTHING`, f.ID, f.Kind, f.Entity, f.Day, pricingStamp(f.At), f.Coverage, f.Health, f.Raw, f.Hash)
	if err != nil {
		return err
	}
	for _, forecast := range forecasts {
		if err = validatePricingForecast(forecast); err != nil {
			return err
		}
		if err = savePricingForecastTx(ctx, tx, forecast); err != nil {
			return err
		}
	}
	return tx.Commit()
}
