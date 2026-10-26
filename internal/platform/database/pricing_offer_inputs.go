package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingExposureInputs(ctx context.Context, q pricingReader, user string, now time.Time) (pricing.ResponseObservation, int, string, error) {
	var raw, watermark string
	err := q.QueryRowContext(ctx, `SELECT features_json,watermark FROM pricing_features WHERE entity_type='user' AND entity_id=? AND watermark<=? AND watermark>=? ORDER BY day DESC LIMIT 1`, user, pricingStamp(now), pricingStamp(now.Add(-48*time.Hour))).Scan(&raw, &watermark)
	if errors.Is(err, sql.ErrNoRows) {
		return pricing.ResponseObservation{}, 0, "", nil
	}
	if err != nil {
		return pricing.ResponseObservation{}, 0, "", err
	}
	var f struct {
		ActiveDays int `json:"activeDays30"`
		Renewals   int `json:"renewals30"`
	}
	if err = json.Unmarshal([]byte(raw), &f); err != nil {
		return pricing.ResponseObservation{}, 0, "", err
	}
	return pricing.ResponseObservation{Activity: float64(f.ActiveDays) / 30, Renewals: float64(min(3, f.Renewals)) / 3}, 1, watermark, nil
}
