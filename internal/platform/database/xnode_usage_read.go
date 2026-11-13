package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const maxUsageHistoryRows = 16384

func (s *Store) XNodeUsageHistory(ctx context.Context, owner string, from, through time.Time) ([]xnode.UsageHistoryRow, xnode.UsageCoverage, error) {
	coverage := xnode.UsageCoverage{GapCount: "0"}
	if owner == "" || through.Before(from) || through.Sub(from) > xnode.PAYGWindow+time.Minute {
		return nil, coverage, xnode.ErrInvalid
	}
	lower, upper := nativeUsageTimeBounds(from, through)
	var floor sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(started_at) FROM xnode_usage_history_state`).Scan(&floor); err != nil {
		return nil, coverage, err
	}
	if floor.Valid {
		value := floor.Time.UTC()
		coverage.Since = &value
	}
	if err := s.db.QueryRowContext(ctx, `WITH nodes AS (
 SELECT DISTINCT node_id FROM xnode_usage_minutes WHERE owner_id=$1
 UNION SELECT p.node_id FROM xnode_period_nodes p JOIN xnode_identities i ON i.proxy_user_id=(SELECT proxy_user_id FROM xnode_periods WHERE id=p.period_id) WHERE i.user_id=$1
 UNION SELECT p.node_id FROM xnode_period_nodes p JOIN hourly_enrollments e ON e.period_id=p.period_id WHERE e.owner_id=$1
 UNION SELECT node_id FROM xnode_payg_accounts WHERE user_id=$1
 ) SELECT COUNT(*)::text FROM xnode_usage_coverage WHERE node_id IN(SELECT node_id FROM nodes) AND observed_at>=$2 AND observed_at<=$3`, owner, lower, upper).Scan(&coverage.GapCount); err != nil {
		return nil, coverage, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT h.billing_mode,h.reference_id,h.node_id,COALESCE(n.name,h.node_id),COALESCE(n.country_code,''),date_trunc('hour',h.minute,'UTC'),SUM(h.upload_bytes)::text,SUM(h.download_bytes)::text FROM xnode_usage_minutes h LEFT JOIN xnode_nodes n ON n.id=h.node_id WHERE h.owner_id=$1 AND h.minute>=$2 AND h.minute<=$3 GROUP BY h.billing_mode,h.reference_id,h.node_id,n.name,n.country_code,date_trunc('hour',h.minute,'UTC') ORDER BY h.node_id,h.billing_mode,h.reference_id,date_trunc('hour',h.minute,'UTC') LIMIT $4`, owner, lower, upper, maxUsageHistoryRows+1)
	if err != nil {
		return nil, coverage, err
	}
	defer rows.Close()
	result := []xnode.UsageHistoryRow{}
	for rows.Next() {
		var row xnode.UsageHistoryRow
		if err = rows.Scan(&row.BillingMode, &row.ReferenceID, &row.NodeID, &row.Name, &row.CountryCode, &row.Hour, &row.UploadBytes, &row.DownloadBytes); err != nil {
			return nil, coverage, err
		}
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, coverage, err
	}
	coverage.Truncated = len(result) > maxUsageHistoryRows
	result = result[:min(len(result), maxUsageHistoryRows)]
	coverage.Complete = coverage.Since != nil && !coverage.Since.After(from) && coverage.GapCount == "0" && !coverage.Truncated
	return result, coverage, nil
}

// Typed native history retains microseconds. Parameter conversion must never
// widen an inclusive nanosecond request to include an adjacent stored instant.
func nativeUsageTimeBounds(from, through time.Time) (time.Time, time.Time) {
	lower := from.UTC().Truncate(time.Microsecond)
	if lower.Before(from) {
		lower = lower.Add(time.Microsecond)
	}
	return lower, through.UTC().Truncate(time.Microsecond)
}
