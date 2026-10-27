package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingModelSupportTx(ctx context.Context, q pricingReader, id string) (*pricing.PriceSupport, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT diagnostics_json FROM pricing_model_versions WHERE id=? AND frozen=0`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return &pricing.PriceSupport{}, nil
	}
	if err != nil {
		return nil, err
	}
	var evidence struct {
		Support pricing.PriceSupport `json:"support"`
	}
	if err = json.Unmarshal([]byte(raw), &evidence); err != nil {
		return nil, err
	}
	return &evidence.Support, nil
}

func pricingResponseSupport(samples []pricing.ResponseObservation) pricing.PriceSupport {
	if len(samples) < 20 {
		return pricing.ObservedSupport(samples)
	}
	// Future holdout observations cannot expand the training price range.
	support := pricing.ObservedSupport(samples[:len(samples)*4/5])
	support.Samples = len(samples)
	return support
}
