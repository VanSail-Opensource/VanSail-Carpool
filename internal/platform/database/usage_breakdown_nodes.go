package database

import (
	"context"
	"database/sql"
	"math/big"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Aggregate references and modes before limiting nodes. Independent charge and
// traffic aggregates avoid multiplying receipts by minute samples.
const usageBreakdownNodesSQL = `WITH raw_traffic AS (
 SELECT node_id,SUM(upload_bytes) up,SUM(download_bytes) down,
 SUM(CASE WHEN billing_mode='combo' AND reference_id=$5 THEN upload_bytes+download_bytes ELSE 0 END) legacy
 FROM xnode_usage_minutes WHERE owner_id=$1 AND minute>=$6 AND minute<=$4 AND observed_through>=$8
 AND (billing_mode<>'hourly' OR account_generation=$2) GROUP BY node_id
 UNION ALL
 SELECT node_id,SUM(upload_bytes),SUM(download_bytes),0
 FROM xnode_payg_node_minutes WHERE user_id=$1 AND minute>=$7 AND minute<=$4 GROUP BY node_id
), traffic AS (
 SELECT node_id,SUM(up) up,SUM(down) down,SUM(legacy) legacy FROM raw_traffic GROUP BY node_id
), costs AS (
 SELECT a.node_id,SUM(c.blocks::numeric*c.unit_price_minor::numeric) actual
 FROM xnode_payg_charges c JOIN ledger_entries l ON l.id=c.ledger_id
 JOIN xnode_payg_accounts a ON a.proxy_user_id=c.proxy_user_id
 WHERE a.user_id=$1 AND l.user_id=$1 AND l.kind='payg_gb'
 AND l.created_at::timestamptz>=$3 AND l.created_at::timestamptz<=$4 GROUP BY a.node_id
), ids AS (SELECT node_id FROM traffic UNION SELECT node_id FROM costs)
 SELECT i.node_id,COALESCE(n.name,i.node_id),COALESCE(n.country_code,''),
 COALESCE(t.up,0)::text,COALESCE(t.down,0)::text,COALESCE(t.legacy,0)::text,
 COALESCE(c.actual,0)::text,COALESCE(SUM(t.legacy) OVER(),0)::text
 FROM ids i LEFT JOIN traffic t ON t.node_id=i.node_id LEFT JOIN costs c ON c.node_id=i.node_id
 LEFT JOIN xnode_nodes n ON n.id=i.node_id ORDER BY i.node_id COLLATE "C"`

func readUsageBreakdownNodesTx(ctx context.Context, tx *sql.Tx, scope usageBreakdownScope, legacy usageLegacy, unallocated *big.Int, result *model.UsageBreakdown) error {
	rows, err := tx.QueryContext(ctx, usageBreakdownNodesSQL, scope.owner, scope.generation, scope.from, scope.to, legacy.id, scope.rawFrom, scope.paygFrom, scope.lifecycle)
	if err != nil {
		return err
	}
	defer rows.Close()
	payg, totalLegacy := new(big.Int), new(big.Int)
	omittedUp, omittedDown, omittedCost := new(big.Int), new(big.Int), new(big.Int)
	denominator := big.NewInt(1)
	trafficPositive, costPositive := uint64(0), uint64(0)
	for rows.Next() {
		var node model.UsageBreakdownNode
		var rawLegacy, rawActual, rawTotal string
		if err = rows.Scan(&node.NodeID, &node.Name, &node.CountryCode, &node.UploadBytes, &node.DownloadBytes, &rawLegacy, &rawActual, &rawTotal); err != nil {
			return err
		}
		values := make([]*big.Int, 0, 5)
		for _, raw := range []string{node.UploadBytes, node.DownloadBytes, rawLegacy, rawActual, rawTotal} {
			value, parseErr := usageUnsigned(raw)
			if parseErr != nil {
				return parseErr
			}
			values = append(values, value)
		}
		up, down, nodeLegacy, actual := values[0], values[1], values[2], values[3]
		totalLegacy.Set(values[4])
		payg.Add(payg, actual)
		numerator := new(big.Int).Set(actual)
		if legacy.status == model.UsageLegacyAvailable && totalLegacy.Sign() > 0 {
			denominator.Mul(legacy.denominator, totalLegacy)
			numerator.Mul(actual, denominator)
			numerator.Add(numerator, new(big.Int).Mul(legacy.numerator, nodeLegacy))
		}
		if up.Sign() > 0 || down.Sign() > 0 {
			trafficPositive++
		}
		if numerator.Sign() > 0 {
			costPositive++
		}
		node.CostMinor = usageRational(numerator, denominator)
		if len(result.Nodes) < model.MaxUsageBreakdownNodes {
			result.Nodes = append(result.Nodes, node)
			continue
		}
		result.Coverage.Truncated = true
		omittedUp.Add(omittedUp, up)
		omittedDown.Add(omittedDown, down)
		omittedCost.Add(omittedCost, numerator)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	result.Omitted = model.UsageBreakdownTotals{UploadBytes: omittedUp.String(), DownloadBytes: omittedDown.String(), CostMinor: usageRational(omittedCost, denominator)}
	result.PositiveNodes = model.UsageBreakdownPositiveNodes{Traffic: strconv.FormatUint(trafficPositive, 10), Cost: strconv.FormatUint(costPositive, 10)}
	result.Cost = usageCostSummary(payg, unallocated, legacy, totalLegacy)
	return nil
}
