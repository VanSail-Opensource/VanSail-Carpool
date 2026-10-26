package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func (s *Store) retryAttempt_pricingOptimizationSnapshot(ctx context.Context, kind, product string, m pricing.ResponseModel, now time.Time) (pricing.OptimizationInputs, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return pricing.OptimizationInputs{}, err
	}
	defer tx.Rollback()
	fixed, err := pricingReference(ctx, tx, kind, product)
	if err != nil {
		return pricing.OptimizationInputs{}, err
	}
	p, err := loadPricingPolicy(ctx, tx, kind, product, fixed)
	if err != nil {
		return pricing.OptimizationInputs{}, err
	}
	in, err := pricingOptimizationInputsTx(ctx, tx, p, m, now)
	if err != nil {
		return in, err
	}
	current, e := approvedPricingTx(ctx, tx, p, fixed, now)
	if e == nil {
		in.Current = current
	}
	return in, nil
}
