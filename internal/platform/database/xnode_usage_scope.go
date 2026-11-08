package database

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type hourlyUsageKey struct{ enrollment, node string }
type hourlyUsageScope struct {
	owner, period         string
	generation, principal int64
	paid                  bool
}

const hourlyUsageScopeSQL = `SELECT e.id,n.node_id,e.owner_id,e.account_generation,e.proxy_user_id,e.period_id,(e.activated_at IS NOT NULL AND e.paid_through IS NOT NULL AND EXISTS(SELECT 1 FROM hourly_settlements s WHERE s.enrollment_id=e.id)) FROM hourly_enrollments e JOIN xnode_periods p ON p.id=e.period_id AND p.proxy_user_id=e.proxy_user_id JOIN xnode_identities i ON i.proxy_user_id=e.proxy_user_id AND i.service_name='member-hourly-'||e.id JOIN xnode_period_nodes n ON n.period_id=e.period_id`

func hourlyUsageScopeTx(ctx context.Context, tx *sql.Tx, buffer *nativeReportBuffer, key hourlyUsageKey) (hourlyUsageScope, error) {
	if buffer != nil {
		value, exists := buffer.parent.hourlyScopes[key]
		if !exists {
			return value, xnode.ErrInvalid
		}
		return value, nil
	}
	var actual hourlyUsageKey
	var value hourlyUsageScope
	err := tx.QueryRowContext(ctx, hourlyUsageScopeSQL+` WHERE e.id=$1 AND n.node_id=$2`, key.enrollment, key.node).Scan(&actual.enrollment, &actual.node, &value.owner, &value.generation, &value.principal, &value.period, &value.paid)
	if errors.Is(err, sql.ErrNoRows) {
		err = xnode.ErrInvalid
	}
	return value, err
}

// Original scopes are immutable; the existing period locks precede this batched
// read. Each input node/enrollment pair has one bounded lookup, not one SQL call.
func loadHourlyUsageScopes(ctx context.Context, tx *sql.Tx, reports []xnode.TrafficCommit) (map[hourlyUsageKey]hourlyUsageScope, error) {
	keys := map[hourlyUsageKey]bool{}
	for _, report := range reports {
		for _, sample := range report.Report.HourlySamples {
			keys[hourlyUsageKey{sample.EnrollmentID, report.NodeID}] = true
		}
	}
	ordered := make([]hourlyUsageKey, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	slices.SortFunc(ordered, func(a, b hourlyUsageKey) int {
		return slices.Compare([]string{a.enrollment, a.node}, []string{b.enrollment, b.node})
	})
	result := map[hourlyUsageKey]hourlyUsageScope{}
	for start := 0; start < len(ordered); start += trafficSQLRows {
		var enrollments, nodes []string
		for _, key := range ordered[start:min(start+trafficSQLRows, len(ordered))] {
			enrollments, nodes = append(enrollments, key.enrollment), append(nodes, key.node)
		}
		rows, err := tx.QueryContext(ctx, hourlyUsageScopeSQL+` JOIN unnest($1::text[],$2::text[]) AS input(enrollment,node) ON input.enrollment=e.id AND input.node=n.node_id`, enrollments, nodes)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var key hourlyUsageKey
			var value hourlyUsageScope
			if err = rows.Scan(&key.enrollment, &key.node, &value.owner, &value.generation, &value.principal, &value.period, &value.paid); err != nil {
				rows.Close()
				return nil, err
			}
			result[key] = value
		}
		if err = finishCompileRows(rows); err != nil {
			return nil, err
		}
	}
	return result, nil
}
