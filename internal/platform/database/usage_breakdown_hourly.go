package database

import (
	"context"
	"database/sql"
	"math/big"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Enrollment identity is unique, so traffic is joined once. Costs aggregate in
// an independent bank before merging IDs: receipts never fan out over minutes.
const usageBreakdownHourlySQL = `WITH traffic AS (
 SELECT e.group_id,SUM(u.upload_bytes) up,SUM(u.download_bytes) down
 FROM xnode_usage_minutes u JOIN hourly_enrollments e ON e.id=u.reference_id
 AND e.owner_id=u.owner_id AND e.account_generation=u.account_generation
 AND e.proxy_user_id=u.proxy_user_id AND e.period_id=u.period_id
 WHERE u.owner_id=$1 AND u.account_generation=$2 AND u.billing_mode='hourly'
 AND u.minute>=$5 AND u.minute<=$4 AND u.observed_through>=$6
 GROUP BY e.group_id
), costs AS (
 SELECT e.group_id,SUM(s.charged_minor::numeric) actual
 FROM hourly_settlements s JOIN hourly_enrollments e ON e.id=s.enrollment_id
 JOIN ledger_entries l ON l.id=s.ledger_id
 WHERE e.owner_id=$1 AND e.account_generation=$2
 AND l.user_id=$1 AND l.kind='hourly_charge'
 AND l.created_at::timestamptz>=$3 AND l.created_at::timestamptz<=$4
 GROUP BY e.group_id
), ids AS (SELECT group_id FROM traffic UNION SELECT group_id FROM costs)
 SELECT i.group_id,COALESCE(g.name,i.group_id),COALESCE(t.up,0)::text,
 COALESCE(t.down,0)::text,COALESCE(c.actual,0)::text
 FROM ids i LEFT JOIN traffic t ON t.group_id=i.group_id
 LEFT JOIN costs c ON c.group_id=i.group_id LEFT JOIN xnode_groups g ON g.id=i.group_id
 ORDER BY i.group_id COLLATE "C"`

func readUsageBreakdownHourlyTx(ctx context.Context, tx *sql.Tx, scope usageBreakdownScope, result *model.UsageBreakdown) error {
	rows, err := tx.QueryContext(ctx, usageBreakdownHourlySQL, scope.owner, scope.generation, scope.from, scope.to, scope.rawFrom, scope.lifecycle)
	if err != nil {
		return err
	}
	defer rows.Close()
	totalCost, omittedUp, omittedDown, omittedCost := new(big.Int), new(big.Int), new(big.Int), new(big.Int)
	trafficCount, err := usageUnsigned(result.PositiveNodes.Traffic)
	if err != nil {
		return err
	}
	costCount, err := usageUnsigned(result.PositiveNodes.Cost)
	if err != nil {
		return err
	}
	for rows.Next() {
		var squad model.UsageBreakdownHourlySquad
		var cost string
		if err = rows.Scan(&squad.GroupID, &squad.Name, &squad.UploadBytes, &squad.DownloadBytes, &cost); err != nil {
			return err
		}
		values := make([]*big.Int, 0, 3)
		for _, raw := range []string{squad.UploadBytes, squad.DownloadBytes, cost} {
			value, parseErr := usageUnsigned(raw)
			if parseErr != nil {
				return parseErr
			}
			values = append(values, value)
		}
		up, down, actual := values[0], values[1], values[2]
		totalCost.Add(totalCost, actual)
		if up.Sign() > 0 || down.Sign() > 0 {
			trafficCount.Add(trafficCount, big.NewInt(1))
		}
		if actual.Sign() > 0 {
			costCount.Add(costCount, big.NewInt(1))
		}
		squad.CostMinor = usageRational(actual, big.NewInt(1))
		if len(result.Nodes)+len(result.HourlySquads) < model.MaxUsageBreakdownEntries {
			result.HourlySquads = append(result.HourlySquads, squad)
			continue
		}
		result.Coverage.Truncated = true
		omittedUp.Add(omittedUp, up)
		omittedDown.Add(omittedDown, down)
		omittedCost.Add(omittedCost, actual)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	result.PositiveEntries = model.UsageBreakdownPositiveNodes{Traffic: trafficCount.String(), Cost: costCount.String()}
	return addUsageBreakdownHourlyTotals(result, totalCost, omittedUp, omittedDown, omittedCost)
}
