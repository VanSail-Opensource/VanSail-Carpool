package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ListSquadProducts enriches display catalog reads only. Exact checkout admission
// remains transactional and independent of this bounded display calculation.
func (s *Store) ListSquadProducts(ctx context.Context, visibleOnly bool) ([]model.SquadProduct, error) {
	products, err := s.ListSquadProductOverrides(ctx, visibleOnly)
	if err != nil {
		return nil, err
	}
	limited := false
	for _, product := range products {
		limited = limited || (product.StockLimit != nil && product.BillingMode != "payg")
	}
	if !limited {
		return products, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	counts, err := s.squadStockCounts(ctx)
	if err != nil {
		return nil, err
	}
	for i := range products {
		p := &products[i]
		if p.StockLimit != nil && p.BillingMode != "payg" {
			remaining := max(0, *p.StockLimit-counts[p.GroupID])
			p.StockRemaining = &remaining
		}
	}
	return products, nil
}

const squadStockQuery = `WITH selected_purchases AS (
	SELECT ` + purchaseSquadColumns + ` FROM purchases WHERE status IN ('activating','active','queued')
), ` + purchaseSquadMembershipCTE + `
SELECT o.group_id,COUNT(DISTINCT o.user_id) FROM owned_squads o
JOIN squad_product_overrides s ON s.group_id=o.group_id
WHERE s.stock_limit IS NOT NULL AND s.billing_mode<>'payg'
GROUP BY o.group_id`

func (s *Store) squadStockCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, squadStockQuery)
	if err != nil {
		return nil, fmt.Errorf("count squad stock: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var group string
		var count int
		if err := rows.Scan(&group, &count); err != nil {
			return nil, err
		}
		counts[group] = count
	}
	return counts, rows.Err()
}
