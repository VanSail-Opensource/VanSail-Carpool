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

// HourlyStockRemaining projects the shared seat count for every squad that has an
// hourly offer, enabled or disabled, so the member UI can explain a capacity
// refusal even when sales are closed. The limit is billing-mode independent: a
// PAYG-marked product with stock_limit still limits hourly seats. Active hourly
// members keep their own seat, and settlement never consults stock (see
// hourly_billing_settlement.go), so only a new activation can be refused.
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
	hourly := map[string]bool{}
	for _, offer := range offers {
		hourly[offer.GroupID] = true
	}
	counts, err := s.squadStockCounts(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]int{}
	for _, product := range products {
		if !hourly[product.GroupID] {
			continue
		}
		if remaining := sharedStockRemaining(product.StockLimit, counts[product.GroupID]); remaining != nil {
			result[product.GroupID] = *remaining
		}
	}
	return result, nil
}
