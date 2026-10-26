package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingOptimizationInputsTx(ctx context.Context, tx *sql.Tx, p pricing.Policy, m pricing.ResponseModel, now time.Time) (pricing.OptimizationInputs, error) {
	in := pricing.OptimizationInputs{Policy: p, Current: p.FixedPrice, Model: m, ForecastValidated: true}
	nodes, err := pricingNodesTx(ctx, tx, p.ProductType, p.ProductID)
	if err != nil {
		return in, err
	}
	for _, node := range nodes {
		var cost, capacity, demand float64
		var days int
		err = tx.QueryRowContext(ctx, `SELECT cycle_cost_minor,cycle_days,capacity_bytes FROM pricing_cost_metadata WHERE node_id=? AND effective_at<=?`, node, pricingStamp(now)).Scan(&cost, &days, &capacity)
		if errors.Is(err, sql.ErrNoRows) {
			return in, pricing.ModelUnavailable
		}
		if err != nil {
			return in, err
		}
		// Conservatively charge the entire machine burden until administrators
		// supply a defensible allocation; no invented provider-cost observations.
		in.CostMinor += cost / float64(days)
		in.CapacityGB += capacity / float64(days) / 1e9
		if err = tx.QueryRowContext(ctx, `SELECT predicted_p90 FROM pricing_forecast_vintages WHERE entity_type='node' AND entity_id=? AND metric='traffic_bytes' AND generated_at<=? AND generated_at>=? ORDER BY generated_at DESC,target_from LIMIT 1`, node, pricingStamp(now), pricingStamp(now.Add(-48*time.Hour))).Scan(&demand); errors.Is(err, sql.ErrNoRows) {
			return in, pricing.ForecastStale
		} else if err != nil {
			return in, err
		}
		in.DemandGB += demand / 1e9
		validated, e := pricingForecastValidatedTx(ctx, tx, node)
		if e != nil {
			return in, e
		}
		if !validated {
			in.ForecastValidated = false
		}
		var prepaid float64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(credit_bytes),0) FROM xnode_payg_accounts WHERE node_id=? AND credit_started_at>?`, node, pricingStamp(now.Add(-72*time.Hour))).Scan(&prepaid); err != nil {
			return in, err
		}
		in.CommittedGB += prepaid / 1e9 / 3
		var comboCommitment float64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN p.limit_bytes>p.used_bytes THEN p.limit_bytes-p.used_bytes ELSE 0 END),0) FROM xnode_periods p JOIN xnode_period_nodes pn ON pn.period_id=p.id WHERE pn.node_id=? AND p.status IN ('open','closing') AND p.purchase_id IS NOT NULL`, node).Scan(&comboCommitment); err != nil {
			return in, err
		}
		committed := prepaid/1e9/3 + comboCommitment/1e9/float64(days)
		in.CommittedGB += comboCommitment / 1e9 / float64(days)
		pricing.ContextHealth(ctx).SetCapacity(node, demand/1e9+committed > capacity/float64(days)/1e9, now)
	}
	if p.ProductType == "payg" {
		in.SalesUnits = max(0, in.DemandGB-in.CommittedGB)
	} else {
		var exposures int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_offers WHERE product_type=? AND product_id=? AND exposed_at>=?`, p.ProductType, p.ProductID, pricingStamp(now.AddDate(0, 0, -30))).Scan(&exposures); err != nil {
			return in, err
		}
		in.SalesUnits = float64(exposures) / 30
	}
	return in, nil
}
