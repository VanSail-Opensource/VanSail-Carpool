package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func usageBreakdownCoverageTx(ctx context.Context, tx *sql.Tx, scope usageBreakdownScope) (model.UsageCoverage, error) {
	coverage := model.UsageCoverage{GapCount: "0"}
	var floor sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT MIN(started_at) FROM xnode_usage_history_state`).Scan(&floor); err != nil {
		return coverage, err
	}
	if floor.Valid {
		since := floor.Time.UTC()
		lifecycleFloor := scope.lifecycle
		if lifecycleFloor != lifecycleFloor.Truncate(time.Minute) {
			lifecycleFloor = lifecycleFloor.Truncate(time.Minute).Add(time.Minute)
		}
		if lifecycleFloor.After(since) {
			since = lifecycleFloor
		}
		coverage.Since = &since
	}
	// Retain overlapping participants after close even when a dropped report
	// left no minute row. Old lifecycles cannot import their earlier gaps.
	err := tx.QueryRowContext(ctx, `WITH nodes AS (
 SELECT u.node_id FROM xnode_usage_minutes u WHERE u.owner_id=$1 AND u.minute>=$5 AND u.minute<=$4 AND u.observed_through>=$7
 AND (u.billing_mode<>'hourly' OR u.account_generation=$2 AND EXISTS(
 SELECT 1 FROM hourly_enrollments e WHERE e.id=u.reference_id AND e.owner_id=u.owner_id
 AND e.account_generation=u.account_generation AND e.proxy_user_id=u.proxy_user_id AND e.period_id=u.period_id))
 UNION SELECT p.node_id FROM xnode_period_nodes p JOIN xnode_periods a ON a.id=p.period_id
 JOIN xnode_identities i ON i.proxy_user_id=a.proxy_user_id
 WHERE i.user_id=$1 AND a.started_at::timestamptz<=$4
 AND (a.ended_at IS NULL OR a.ended_at::timestamptz>=$3)
 UNION SELECT p.node_id FROM xnode_period_nodes p JOIN hourly_enrollments e ON e.period_id=p.period_id
 JOIN xnode_periods a ON a.id=p.period_id WHERE e.owner_id=$1 AND e.account_generation=$2
 AND a.started_at::timestamptz<=$4 AND (a.ended_at IS NULL OR a.ended_at::timestamptz>=$3)
 UNION SELECT a.node_id FROM xnode_payg_accounts a JOIN xnode_periods p ON p.id=a.period_id
 WHERE a.user_id=$1 AND p.started_at::timestamptz<=$4
 AND (p.ended_at IS NULL OR p.ended_at::timestamptz>=$3)
 UNION SELECT node_id FROM xnode_payg_node_minutes WHERE user_id=$1 AND minute>=$6 AND minute<=$4
 ) SELECT COUNT(*)::text FROM xnode_usage_coverage WHERE node_id IN(SELECT node_id FROM nodes)
 AND observed_at>=$3 AND observed_at<=$4`, scope.owner, scope.generation, scope.from, scope.to, scope.rawFrom, scope.paygFrom, scope.lifecycle).Scan(&coverage.GapCount)
	if err != nil {
		return coverage, err
	}
	return coverage, nil
}
