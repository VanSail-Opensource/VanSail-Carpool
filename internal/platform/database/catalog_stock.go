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
		limited = limited || displayedCatalogStockLimit(product) != nil
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
		if limit := displayedCatalogStockLimit(products[i]); limit != nil {
			products[i].StockRemaining = sharedStockRemaining(limit, counts[products[i].GroupID])
		}
	}
	return products, nil
}

// displayedCatalogStockLimit is the member-catalog display rule. PAYG products
// carry their own metering and are excluded from the fixed-term seat display;
// exact checkout admission still checks any limited squad transactionally.
func displayedCatalogStockLimit(product model.SquadProduct) *int {
	if product.StockLimit == nil || product.BillingMode == "payg" {
		return nil
	}
	return product.StockLimit
}

// sharedStockRemaining projects display stock from the one shared seat count
// (distinct members in owned_squads plus active hourly_enrollments). This helper
// is the only place the max(0, limit-count) arithmetic exists, so the member
// catalog and the hourly view cannot diverge on which members count or on the
// limit. A nil limit means the squad is unlimited; callers choose which billing
// modes they display.
func sharedStockRemaining(limit *int, sharedCount int) *int {
	if limit == nil {
		return nil
	}
	remaining := max(0, *limit-sharedCount)
	return &remaining
}

const squadStockQuery = `WITH selected_purchases AS (
	SELECT ` + purchaseSquadColumns + ` FROM purchases WHERE status IN ('activating','active','queued')
), ` + purchaseSquadMembershipCTE + `
SELECT o.group_id,COUNT(DISTINCT o.user_id) FROM (
 SELECT group_id,user_id FROM owned_squads
 UNION SELECT group_id,owner_id FROM hourly_enrollments WHERE status='active'
) o
JOIN squad_product_overrides s ON s.group_id=o.group_id
WHERE s.stock_limit IS NOT NULL
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
