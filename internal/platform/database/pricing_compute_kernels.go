package database

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func (s *Store) pricingQuantiles(ctx context.Context, samples []float64) (compute.Quantiles, error) {
	result, err := s.PricingCalculate(ctx, compute.Task{Kind: "forecast_quantiles_v1", Samples: samples})
	if err != nil {
		return compute.Quantiles{}, err
	}
	if result.Quantiles == nil {
		return compute.Quantiles{}, ErrConflict
	}
	return *result.Quantiles, nil
}

func (s *Store) pricingFitResponse(ctx context.Context, rows []pricing.ResponseObservation) (pricing.ResponseModel, error) {
	task := compute.Task{Kind: "response_fit_v1", Observations: rows}
	// Retain the newest observations within both the sample and wire budgets.
	if len(task.Observations) > 2048 {
		task.Observations = task.Observations[len(task.Observations)-2048:]
	}
	for len(task.Observations) > 0 {
		raw, e := json.Marshal(task)
		if e != nil {
			return pricing.ResponseModel{}, e
		}
		if len(raw) < compute.MaxInputBytes-2048 {
			break
		}
		task.Observations = task.Observations[max(1, len(task.Observations)/8):]
	}
	result, err := s.PricingCalculate(ctx, task)
	if err != nil {
		return pricing.ResponseModel{}, err
	}
	if result.Model == nil {
		return pricing.ResponseModel{}, ErrConflict
	}
	return *result.Model, nil
}
