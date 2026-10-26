package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
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
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(actual_paid_minor),0),COUNT(actual_paid_minor) FROM pricing_cost_versions WHERE effective_at>=? AND effective_at<?`, pricingStamp(from), pricingStamp(now)).Scan(&paid, &costCount); err != nil {
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
	var dailyCost sql.NullFloat64
	var totalNodes, knownNodes int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(c.node_id),SUM(c.cycle_cost_minor*1.0/c.cycle_days) FROM xnode_nodes n LEFT JOIN pricing_cost_metadata c ON c.node_id=n.id WHERE n.enabled=1`).Scan(&totalNodes, &knownNodes, &dailyCost); err != nil {
		return nil, err
	}
	var forecastRevenue, forecastMargin any
	if forecast.Valid {
		forecastRevenue = fmt.Sprint(int64(math.Round(forecast.Float64)))
		if dailyCost.Valid && knownNodes == totalNodes {
			forecastMargin = fmt.Sprint(int64(math.Round(forecast.Float64 - dailyCost.Float64)))
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
	return map[string]any{"from": from, "until": now, "netRevenueMinor": fmt.Sprint(revenue), "recordedCostMinor": cost, "recordedMarginMinor": margin, "providerPaymentRecords": costCount, "forecastRevenueMinor": forecastRevenue, "forecastMarginMinor": forecastMargin, "forecastCalibration": "UNVALIDATED", "costNodes": knownNodes, "totalNodes": totalNodes, "dynamicProducts": dynamic, "configuredProducts": configured, "pendingJobs": pending, "failedJobs": failed, "interventions30": interventions, "manualDiscountMinor30": fmt.Sprint(discount)}, nil
}
