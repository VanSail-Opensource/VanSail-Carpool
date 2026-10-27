package database

import (
	"context"
	"errors"
	"time"
)

// Minute publication has its own bounded pass; training is reused for its daily
// model version. Existing prepaid credits and unit-price receipts are untouched.
func (s *Store) PricingPAYGBatch(ctx context.Context, now time.Time) error {
	_, err := s.PricingPAYGBatchAfter(ctx, now, "")
	return err
}

// PricingPAYGBatchAfter advances the attempted product even if its compute is pending.
func (s *Store) PricingPAYGBatchAfter(ctx context.Context, now time.Time, after string) (next string, resultErr error) {
	next = after
	rows, err := s.db.QueryContext(ctx, `SELECT product_id FROM pricing_product_settings p WHERE product_type='payg' AND dynamic_enabled=1 AND purchase_pricing_mode='DYNAMIC' AND product_id>? AND NOT EXISTS(SELECT 1 FROM pricing_decisions d WHERE d.product_type='payg' AND d.product_id=p.product_id AND d.policy_version>=p.version AND d.generated_at>=?) ORDER BY product_id LIMIT 4`, after, pricingStamp(now.UTC().Truncate(time.Minute)))
	if err != nil {
		return next, err
	}
	products := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return next, err
		}
		products = append(products, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return next, err
	}
	for _, id := range products {
		next = id
		if err = s.PricingProductForecasts(ctx, "payg", id, now); err == nil {
			err = s.PricingOptimizeProduct(ctx, "payg", id, now)
		}
		if err != nil && !errors.Is(err, ErrPricingComputePending) {
			return next, err
		}
	}
	if len(products) < 4 {
		next = ""
	}
	return next, nil
}
