package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"sort"
	"strings"
	"time"
)

func pricingOptimizationInputsTx(ctx context.Context, tx *sql.Tx, p pricing.Policy, m pricing.ResponseModel, now time.Time) (pricing.OptimizationInputs, error) {
	in := pricing.OptimizationInputs{Policy: p, Current: p.FixedPrice, Model: m, ForecastValidated: true}
	nodes, err := pricingNodesTx(ctx, tx, p.ProductType, p.ProductID)
	if err != nil {
		return in, err
	}
	allocations, err := pricingAllocationsTx(ctx, tx, p.ProductType, p.ProductID, now)
	if err != nil {
		return in, err
	}
	day := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	rows, err := tx.QueryContext(ctx, `SELECT c.node_id,c.cycle_cost_minor,c.cycle_days,c.capacity_bytes,c.charge_mode,c.hourly_rate_micros,c.traffic_rate_micros,c.capacity_unlimited,c.version,
 (SELECT f.predicted_p90 FROM pricing_forecast_vintages f WHERE f.entity_type='node' AND f.entity_id=c.node_id AND f.metric='traffic_bytes' AND f.model_version='empirical-daily-v1' AND f.horizon_seconds=86400 AND f.target_from=? AND f.target_until=? AND f.generated_at<=? AND f.generated_at>=? ORDER BY f.generated_at DESC LIMIT 1),
 (SELECT COALESCE(SUM(a.credit_bytes),0) FROM xnode_payg_accounts a WHERE a.node_id=c.node_id AND a.credit_started_at>?),
 (SELECT COALESCE(SUM(CASE WHEN t.limit_bytes>t.used_bytes THEN t.limit_bytes-t.used_bytes ELSE 0 END),0) FROM xnode_periods t JOIN xnode_period_nodes pn ON pn.period_id=t.id WHERE pn.node_id=c.node_id AND t.status IN ('open','closing') AND t.purchase_id IS NOT NULL),
 (SELECT COALESCE(SUM(a.credit_bytes),0) FROM xnode_payg_accounts a WHERE a.node_id=c.node_id AND a.group_id=? AND a.credit_started_at>?)
 FROM pricing_cost_metadata c WHERE c.effective_at<=? ORDER BY c.node_id LIMIT 4097`, pricingStamp(day), pricingStamp(day.Add(24*time.Hour)), pricingStamp(now), pricingStamp(now.Add(-48*time.Hour)), pricingStamp(now.Add(-xnode.PAYGWindow)), p.ProductID, pricingStamp(now.Add(-xnode.PAYGWindow)), pricingStamp(now))
	if err != nil {
		return in, err
	}
	type machine struct {
		cost, days, capacity, prepaid, combo, productCredit float64
		hourly, traffic                                     float64
		mode                                                string
		unlimited, version                                  int64
		demand                                              sql.NullFloat64
	}
	machines := map[string]machine{}
	for rows.Next() {
		var node string
		var n machine
		if err = rows.Scan(&node, &n.cost, &n.days, &n.capacity, &n.mode, &n.hourly, &n.traffic, &n.unlimited, &n.version, &n.demand, &n.prepaid, &n.combo, &n.productCredit); err != nil {
			rows.Close()
			return in, err
		}
		if len(machines) >= 4096 {
			rows.Close()
			return in, pricing.StoragePressure
		}
		machines[node] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return in, err
	}
	sort.Strings(nodes)
	var evidence strings.Builder
	finite := 0
	for _, node := range nodes {
		n, ok := machines[node]
		if !ok || n.days <= 0 || (n.unlimited == 0 && n.capacity <= 0) {
			return in, pricing.ModelUnavailable
		}
		if !n.demand.Valid {
			return in, pricing.ForecastStale
		}
		a := allocations[node]
		if a.Total <= 0 || a.Product <= 0 {
			return in, pricing.ModelUnavailable
		}
		share := a.Product / a.Total
		fmt.Fprintf(&evidence, "%s:%d;", node, n.version)
		fixed := n.cost / n.days
		if n.mode == "hourly" {
			fixed = n.hourly * 24 / 10000
		}
		in.CostMinor += (fixed + n.demand.Float64/float64(xnode.PAYGGB)*n.traffic/10000) * share
		capacity := n.capacity / n.days
		commitment := n.prepaid/3 + n.combo/n.days
		// Forecast total consumption includes paid traffic; reserve the larger estimate.
		required := max(n.demand.Float64, commitment)
		unsafe := n.unlimited == 0 && required > capacity
		if health := pricing.ContextHealth(ctx); health != nil {
			health.SetCapacity(node, unsafe, now)
		}
		if unsafe {
			return in, pricing.CapacityUnsafe
		}
		if n.unlimited == 0 {
			finite++
			in.CapacityGB += capacity / float64(xnode.PAYGGB)
			in.CommittedGB += commitment / float64(xnode.PAYGGB)
			in.DemandGB += max(0, required-commitment) / float64(xnode.PAYGGB)
		}
		if p.ProductType == "payg" {
			in.SalesUnits += max(0, n.demand.Float64*share-n.productCredit/3) / float64(xnode.PAYGGB)
		}
		validated, e := pricingForecastValidatedTx(ctx, tx, node)
		if e != nil {
			return in, e
		}
		in.ForecastValidated = in.ForecastValidated && validated
	}
	in.CapacityUnbounded = finite == 0 && len(nodes) > 0
	in.CostEvidence = evidence.String()
	if p.ProductType != "payg" {
		var exposures int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT user_id,SUBSTR(exposed_at,1,10) AS day FROM pricing_offers WHERE product_type=? AND product_id=? AND exposed_at>=? GROUP BY user_id,SUBSTR(exposed_at,1,10)) exposures`, p.ProductType, p.ProductID, pricingStamp(now.AddDate(0, 0, -30))).Scan(&exposures); err != nil {
			return in, err
		}
		in.SalesUnits = float64(exposures) / 30
	}
	if in.SalesUnits <= 0 {
		return in, pricing.ModelUnavailable
	}
	return in, nil
}
