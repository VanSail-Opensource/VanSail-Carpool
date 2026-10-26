package database

import (
	"context"
	"errors"
	"time"
)

// Revenue forecasting includes fixed products without activating their pricing.
func (s *Store) PricingRevenueForecastBatch(ctx context.Context, now time.Time, cursor string) (string, error) {
	release, err := s.acquirePricingCompute()
	if err != nil {
		return cursor, err
	}
	defer release()
	rows, err := s.db.QueryContext(ctx, `SELECT kind,id FROM (SELECT 'combo' AS kind,id FROM combos WHERE active=1 UNION ALL SELECT 'payg' AS kind,group_id AS id FROM squad_product_overrides WHERE billing_mode='payg') p WHERE kind||':'||id>? AND NOT EXISTS(SELECT 1 FROM pricing_features f WHERE f.entity_type='product' AND f.entity_id=p.kind||':'||p.id AND f.day=?) ORDER BY kind,id LIMIT 4`, cursor, now.UTC().Format("2006-01-02"))
	if err != nil {
		return cursor, err
	}
	type target struct{ kind, id string }
	items := []target{}
	for rows.Next() {
		var p target
		if err = rows.Scan(&p.kind, &p.id); err != nil {
			rows.Close()
			return cursor, err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return cursor, err
	}
	for _, p := range items {
		if err = s.PricingProductForecasts(ctx, p.kind, p.id, now); err != nil {
			if errors.Is(err, ErrPricingComputePending) {
				continue
			}
			return cursor, err
		}
	}
	if len(items) < 4 {
		return "", nil
	}
	last := items[len(items)-1]
	return last.kind + ":" + last.id, nil
}
