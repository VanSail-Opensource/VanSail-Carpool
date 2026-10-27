package database

import (
	"context"
	"database/sql"
	"math"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
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
	day := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	rows, err := tx.QueryContext(ctx, `SELECT c.node_id,c.cycle_cost_minor,c.cycle_days,c.charge_mode,c.hourly_rate_micros,c.traffic_rate_micros, (SELECT f.predicted_p90 FROM pricing_forecast_vintages f WHERE f.entity_type='node' AND f.entity_id=c.node_id AND f.metric='traffic_bytes' AND f.model_version='empirical-daily-v1' AND f.horizon_seconds=86400 AND f.target_from=? AND f.target_until=? AND f.generated_at<=? AND f.generated_at>=? ORDER BY f.generated_at DESC LIMIT 1) FROM pricing_cost_metadata c WHERE c.effective_at<=?`, pricingStamp(day), pricingStamp(day.Add(24*time.Hour)), pricingStamp(now), pricingStamp(now.Add(-48*time.Hour)), pricingStamp(now))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	overhead := 0.0
	count := 0
	for rows.Next() {
		var id string
		count++
		var cost, days, hourly, traffic float64
		var mode string
		var demand sql.NullFloat64
		if err = rows.Scan(&id, &cost, &days, &mode, &hourly, &traffic, &demand); err != nil {
			return "", err
		}
		a := allocations[id]
		share := 0.0
		if a.Total > 0 {
			share = a.Allocated / a.Total
		}
		fixed := cost / days
		if mode == "hourly" {
			fixed = hourly * 24 / 10000
		}
		if traffic > 0 && !demand.Valid {
			return "", nil
		}
		variable := 0.0
		if demand.Valid {
			variable = demand.Float64 / float64(xnode.PAYGGB) * traffic / 10000
		}
		overhead += (fixed + variable) * (1 - min(1, max(0, share)))
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
	return strconv.FormatInt(int64(math.Round(overhead)), 10), nil
}
