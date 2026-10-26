package database

import (
	"context"
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
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(entity_type,entity_id,day) DO NOTHING`, f.ID, f.Kind, f.Entity, f.Day, pricingStamp(f.At), f.Coverage, f.Health, f.Raw, f.Hash)
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
