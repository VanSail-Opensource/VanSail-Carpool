package database

import (
	"context"
	"database/sql"
	"slices"
	"time"
)

// Mutating legacy/hourly admissions share these product rows. Pure quote paths
// deliberately keep their advisory check unlocked. Take owner/wallet locks first.
func lockSquadStockTx(ctx context.Context, tx *sql.Tx, groups []string) error {
	ordered := slices.Clone(groups)
	slices.Sort(ordered)
	ordered = slices.Compact(ordered)
	for _, group := range ordered {
		var ignored string
		err := tx.QueryRowContext(ctx, `SELECT id FROM xnode_groups WHERE id=$1 FOR SHARE`, group).Scan(&ignored)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}
	for _, group := range ordered {
		var ignored string
		err := tx.QueryRowContext(ctx, `SELECT group_id FROM squad_product_overrides WHERE group_id=$1 FOR UPDATE`, group).Scan(&ignored)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}
	return nil
}

// Hourly offers have independent stock semantics even when the grandfathered
// product's billing mode is PAYG. Do not change the ordinary PAYG catalog view.
func (s *Store) HourlyStockRemaining(ctx context.Context) (map[string]int, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	products, err := s.ListSquadProductOverrides(ctx, false)
	if err != nil {
		return nil, err
	}
	offers, err := s.ListHourlyOffers(ctx)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, offer := range offers {
		enabled[offer.GroupID] = offer.Enabled
	}
	counts, err := s.squadStockCounts(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]int{}
	for _, product := range products {
		if enabled[product.GroupID] && product.StockLimit != nil {
			result[product.GroupID] = max(0, *product.StockLimit-counts[product.GroupID])
		}
	}
	return result, nil
}
