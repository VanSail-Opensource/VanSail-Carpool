package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"time"
)

type pricingNodeAllocation struct{ Total, Allocated, Product float64 }

// Share native period/node usage between its fulfilling products. Services and
// unknown product relationships remain unallocated, never charged to every product.
const pricingAllocationSQL = `WITH native AS (
 SELECT node_id,period_id,SUM(upload_bytes+download_bytes) AS bytes FROM xnode_daily_usage
 WHERE date>=? AND date<? GROUP BY node_id,period_id
), products AS (
 SELECT n.node_id,n.period_id,'combo' AS kind,p.combo_id AS product FROM native n
 JOIN xnode_periods t ON t.id=n.period_id JOIN purchases p ON p.id=t.purchase_id JOIN combos c ON c.id=p.combo_id
 WHERE EXISTS(SELECT 1 FROM xnode_group_bindings b JOIN json_each(COALESCE(p.combo_squad_uuids,c.included_squad_uuids)) j ON j.value=b.group_id WHERE b.node_id=n.node_id)
 UNION ALL SELECT n.node_id,n.period_id,'addon',a.group_id FROM native n
 JOIN xnode_periods t ON t.id=n.period_id JOIN purchase_addons a ON a.purchase_id=t.purchase_id
 WHERE EXISTS(SELECT 1 FROM xnode_group_bindings b WHERE b.group_id=a.group_id AND b.node_id=n.node_id)
 UNION ALL SELECT n.node_id,n.period_id,'payg',a.group_id FROM native n JOIN xnode_payg_accounts a ON a.period_id=n.period_id AND a.node_id=n.node_id
), counts AS (SELECT node_id,period_id,COUNT(*) AS count FROM products GROUP BY node_id,period_id),
 totals AS (SELECT node_id,SUM(bytes) AS total FROM native GROUP BY node_id)
 SELECT t.node_id,t.total,COALESCE(SUM(n.bytes*1.0/c.count),0),
 COALESCE(SUM(CASE WHEN p.kind=? AND p.product=? THEN n.bytes*1.0/c.count ELSE 0 END),0)
 FROM totals t LEFT JOIN native n ON n.node_id=t.node_id
 LEFT JOIN products p ON p.node_id=n.node_id AND p.period_id=n.period_id
 LEFT JOIN counts c ON c.node_id=n.node_id AND c.period_id=n.period_id GROUP BY t.node_id,t.total LIMIT 4097`

func pricingAllocationsTx(ctx context.Context, tx *sql.Tx, kind, product string, now time.Time) (map[string]pricingNodeAllocation, error) {
	day := now.UTC().Truncate(24 * time.Hour)
	rows, err := tx.QueryContext(ctx, pricingAllocationSQL, day.AddDate(0, 0, -30).Format(time.DateOnly), day.Format(time.DateOnly), kind, product)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]pricingNodeAllocation{}
	for rows.Next() {
		var node string
		var a pricingNodeAllocation
		if err = rows.Scan(&node, &a.Total, &a.Allocated, &a.Product); err != nil {
			return nil, err
		}
		if len(out) >= 4096 {
			return nil, pricing.StoragePressure
		}
		a.Allocated, a.Product = min(a.Total, a.Allocated), min(a.Total, a.Product)
		out[node] = a
	}
	return out, rows.Err()
}
