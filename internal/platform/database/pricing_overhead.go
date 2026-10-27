package database

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"time"
)

func (s *Store) retryAttempt_pricingUnallocatedOverhead(ctx context.Context, now time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	allocations, err := pricingAllocationsTx(ctx, tx, "", "", now)
	if err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `SELECT node_id,cycle_cost_minor,cycle_days FROM pricing_cost_metadata WHERE effective_at<=?`, pricingStamp(now))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	overhead := 0.0
	count := 0
	for rows.Next() {
		var id string
		count++
		var cost, days float64
		if err = rows.Scan(&id, &cost, &days); err != nil {
			return "", err
		}
		a := allocations[id]
		share := 0.0
		if a.Total > 0 {
			share = a.Allocated / a.Total
		}
		overhead += cost / days * (1 - min(1, max(0, share)))
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
	return strconv.FormatInt(int64(math.Round(overhead)), 10), nil
}
