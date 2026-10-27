package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// PricingSummary reports ledger facts separately from estimated future receipts
// and costs. Missing provider payments never become an invented zero cost.
func (s *Store) PricingSummary(ctx context.Context, now time.Time) (map[string]any, error) {
	return pricingCached(ctx, s, "summary", time.Minute, func() (map[string]any, error) { return s.pricingSummary(ctx, now) })
}
func (s *Store) pricingSummary(ctx context.Context, now time.Time) (map[string]any, error) {
	from := now.AddDate(0, 0, -30)
	var revenue, paid int64
	var costCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(-SUM(delta_txb_minor),0) FROM ledger_entries WHERE created_at>=? AND created_at<? AND kind IN ('purchase_debit','purchase_addon_debit','automatic_renewal','payg_gb','purchase_cancellation','admin_entitlement_cancellation')`, pricingStamp(from), pricingStamp(now)).Scan(&revenue); err != nil {
		return nil, err
	}
	// Actual payments have a recording time, not a separate payment date. The
	// metadata effective date describes the rate and may be much older.
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(actual_paid_minor),0),COUNT(actual_paid_minor) FROM pricing_cost_versions WHERE recorded_at>=? AND recorded_at<?`, pricingStamp(from), pricingStamp(now)).Scan(&paid, &costCount); err != nil {
		return nil, err
	}
	var cost, margin any
	if costCount > 0 {
		cost, margin = fmt.Sprint(paid), fmt.Sprint(revenue-paid)
	}
	var forecast sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT SUM(f.predicted_mean) FROM pricing_forecast_vintages f WHERE f.entity_type='product' AND f.metric='gross_revenue_minor' AND f.target_from=? AND f.model_version='empirical-receipts-v1' AND f.generated_at=(SELECT MAX(x.generated_at) FROM pricing_forecast_vintages x WHERE x.entity_type=f.entity_type AND x.entity_id=f.entity_id AND x.metric=f.metric AND x.target_from=f.target_from AND x.model_version=f.model_version)`, pricingStamp(now.UTC().Truncate(24*time.Hour).Add(24*time.Hour))).Scan(&forecast); err != nil {
		return nil, err
	}
	dailyCost, knownNodes, totalNodes, costsComplete, err := s.pricingNextDayCosts(ctx, now)
	if err != nil {
		return nil, err
	}
	var forecastRevenue, forecastMargin any
	if forecast.Valid {
		forecastRevenue = fmt.Sprint(int64(math.Round(forecast.Float64)))
		if costsComplete && knownNodes > 0 && knownNodes == totalNodes {
			forecastMargin = fmt.Sprint(int64(math.Round(forecast.Float64 - dailyCost)))
		}
	}
	var dynamic, configured int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(dynamic_enabled),0),COUNT(*) FROM pricing_product_settings`).Scan(&dynamic, &configured); err != nil {
		return nil, err
	}
	var pending, failed int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN status IN ('pending','processing') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0) FROM outbox_jobs WHERE kind LIKE 'pricing_%'`).Scan(&pending, &failed); err != nil {
		return nil, err
	}
	var interventions int
	var discount int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_admin_overrides WHERE created_at>=?`, pricingStamp(from)).Scan(&interventions); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(discount_minor),0) FROM pricing_discount_spend WHERE status='CONSUMED' AND created_at>=?`, pricingStamp(from)).Scan(&discount); err != nil {
		return nil, err
	}
	overhead, err := s.pricingUnallocatedOverhead(ctx, now)
	if err != nil {
		return nil, err
	}
	return map[string]any{"unallocatedDailyCostMinor": overhead, "from": from, "until": now, "netRevenueMinor": fmt.Sprint(revenue), "recordedCostMinor": cost, "recordedMarginMinor": margin, "providerPaymentRecords": costCount, "forecastRevenueMinor": forecastRevenue, "forecastMarginMinor": forecastMargin, "forecastCalibration": "UNVALIDATED", "costNodes": knownNodes, "totalNodes": totalNodes, "dynamicProducts": dynamic, "configuredProducts": configured, "pendingJobs": pending, "failedJobs": failed, "interventions30": interventions, "manualDiscountMinor30": fmt.Sprint(discount)}, nil
}

// pricingNextDayCosts refuses to invent traffic costs when the node forecast is missing.
func (s *Store) pricingNextDayCosts(ctx context.Context, now time.Time) (float64, int, int, bool, error) {
	day := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	rows, err := s.db.QueryContext(ctx, `SELECT c.node_id,c.cycle_cost_minor,c.cycle_days,c.charge_mode,c.hourly_rate_micros,c.traffic_rate_micros,
 (SELECT f.predicted_p90 FROM pricing_forecast_vintages f WHERE f.entity_type='node' AND f.entity_id=c.node_id AND f.metric='traffic_bytes' AND f.model_version='empirical-daily-v1' AND f.horizon_seconds=86400 AND f.target_from=? AND f.target_until=? AND f.generated_at<=? AND f.generated_at>=? ORDER BY f.generated_at DESC LIMIT 1)
 FROM xnode_nodes n LEFT JOIN pricing_cost_metadata c ON c.node_id=n.id AND c.effective_at<=? WHERE n.enabled=1`, pricingStamp(day), pricingStamp(day.Add(24*time.Hour)), pricingStamp(now), pricingStamp(now.Add(-48*time.Hour)), pricingStamp(now))
	if err != nil {
		return 0, 0, 0, false, err
	}
	defer rows.Close()
	total, known := 0, 0
	cost := 0.0
	complete := true
	for rows.Next() {
		var id, mode sql.NullString
		var cycle, days, hourly, traffic, demand sql.NullFloat64
		if err = rows.Scan(&id, &cycle, &days, &mode, &hourly, &traffic, &demand); err != nil {
			return 0, 0, 0, false, err
		}
		total++
		if !id.Valid {
			complete = false
			continue
		}
		known++
		fixed := cycle.Float64 / days.Float64
		if mode.String == "hourly" {
			fixed = hourly.Float64 * 24 / 10000
		}
		if traffic.Float64 > 0 && !demand.Valid {
			complete = false
			continue
		}
		cost += fixed
		if demand.Valid {
			cost += demand.Float64 / float64(xnode.PAYGGB) * traffic.Float64 / 10000
		}
	}
	return cost, known, total, complete, rows.Err()
}
