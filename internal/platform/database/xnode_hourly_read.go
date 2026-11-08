package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const hourlyPrincipalColumns = `e.id,e.owner_id,e.account_generation,e.group_id,COALESCE(e.proxy_user_id,0),COALESCE(e.period_id,''),e.status,e.activated_at,e.paid_through`

func (s *Store) XNodeHourlyPrincipal(ctx context.Context, id string) (xnode.HourlyPrincipal, error) {
	return scanHourlyPrincipal(s.db.QueryRowContext(ctx, `SELECT `+hourlyPrincipalColumns+` FROM hourly_enrollments e WHERE e.id=$1`, id))
}

func scanHourlyPrincipal(row rowScanner) (xnode.HourlyPrincipal, error) {
	var p xnode.HourlyPrincipal
	var activated, paid sql.NullTime
	err := row.Scan(&p.EnrollmentID, &p.OwnerID, &p.AccountGeneration, &p.GroupID, &p.ProxyUserID, &p.PeriodID, &p.Status, &activated, &paid)
	if errors.Is(err, sql.ErrNoRows) {
		return p, xnode.ErrNotFound
	}
	if err == nil && activated.Valid {
		p.ActivatedAt = activated.Time.UTC()
	}
	if err == nil && paid.Valid {
		p.PaidThrough = paid.Time.UTC()
	}
	return p, err
}

// Historical tuples remain queryable; callers independently decide admission.
func (s *Store) XNodeHourlyPrincipals(ctx context.Context, owner, node string) ([]xnode.HourlyPrincipal, error) {
	return s.queryHourlyPrincipals(ctx, `SELECT `+hourlyPrincipalColumns+` FROM hourly_enrollments e JOIN users u ON u.id=e.owner_id AND u.account_generation=e.account_generation WHERE ($1='' OR e.owner_id=$1) AND e.status IN ('preparing','active','funding_suspended') AND NOT EXISTS(SELECT 1 FROM hourly_enrollments newer WHERE newer.owner_id=e.owner_id AND newer.account_generation=e.account_generation AND newer.group_id=e.group_id AND (newer.created_at,newer.id)>(e.created_at,e.id)) AND ($2='' OR EXISTS(SELECT 1 FROM xnode_group_bindings b WHERE b.group_id=e.group_id AND b.node_id=$2)) ORDER BY e.owner_id,e.activated_at,e.id`, owner, node)
}

// Joined current authority for the two-second provider: no per-enrollment
// owner/restriction/period SQL and no financial/live-policy state cache.
func (s *Store) XNodeHourlyPolicyPrincipals(ctx context.Context, now time.Time) ([]xnode.HourlyPrincipal, error) {
	return s.queryHourlyPrincipals(ctx, `SELECT `+hourlyPrincipalColumns+` FROM hourly_enrollments e
 JOIN users u ON u.id=e.owner_id AND u.account_generation=e.account_generation AND u.account_status='active' AND u.onboarding_state='complete'
 JOIN xnode_identities parent ON parent.user_id=u.id AND parent.access_disabled=0
 JOIN xnode_identities i ON i.proxy_user_id=e.proxy_user_id AND i.current_period_id=e.period_id AND i.service_name='member-hourly-'||e.id AND i.access_disabled=0
 JOIN xnode_periods p ON p.id=e.period_id AND p.proxy_user_id=e.proxy_user_id AND p.status='open' AND p.strategy='NO_RESET' AND p.purchase_id IS NULL
 WHERE e.status='active' AND e.paid_through>$1 AND NOT EXISTS(SELECT 1 FROM admin_temporary_bans b WHERE b.user_id=e.owner_id AND b.restored_at IS NULL)
 ORDER BY e.owner_id,e.id`, now.UTC())
}

func (s *Store) queryHourlyPrincipals(ctx context.Context, query string, args ...any) ([]xnode.HourlyPrincipal, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []xnode.HourlyPrincipal{}
	for rows.Next() {
		p, e := scanHourlyPrincipal(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
