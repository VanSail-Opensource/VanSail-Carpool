package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// The callback may only change bounded in-memory admission state. It runs under
// the same users-row lock as activation/renewal/resume and performs no disk or
// network I/O. Native period settlement happens after this transaction returns.
func (s *Store) WithHourlyOwnerLifecycle(ctx context.Context, intent xnode.HourlyLifecycleIntent, mark func() error) ([]xnode.HourlyPrincipal, bool, error) {
	if intent.OwnerID == "" || intent.CurrentGeneration < 0 || intent.ThroughGeneration < 0 || intent.ThroughGeneration > intent.CurrentGeneration || intent.Purge && intent.Resume || !intent.Purge && !intent.Resume && intent.EnrollmentID == "" {
		return nil, false, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var current int64
	var status, role string
	if err = tx.QueryRowContext(ctx, `SELECT account_generation,account_status,role FROM users WHERE id=$1 FOR UPDATE`, intent.OwnerID).Scan(&current, &status, &role); err != nil {
		return nil, false, err
	}
	var allowed bool
	if intent.Purge {
		allowed = current == intent.CurrentGeneration && status == "purging" && role != "admin"
	} else if intent.Resume {
		if current == intent.CurrentGeneration && status == "active" {
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2 AND status='active' AND paid_through>clock_timestamp())`, intent.OwnerID, current).Scan(&allowed)
		}
	} else {
		var newer bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2 AND (status='preparing' OR status='active' AND paid_through>clock_timestamp())) OR EXISTS(SELECT 1 FROM purchases WHERE user_id=$1 AND status IN('active','activating','queued') AND valid_until::timestamptz>clock_timestamp())`, intent.OwnerID, current).Scan(&newer)
		allowed = current == intent.CurrentGeneration && status == "active" && !newer
	}
	if err != nil {
		return nil, false, err
	}
	query := `SELECT ` + hourlyPrincipalColumns + ` FROM hourly_enrollments e WHERE e.owner_id=$1 AND e.account_generation<=$2`
	args := []any{intent.OwnerID, intent.ThroughGeneration}
	if !intent.Purge && !intent.Resume {
		query += ` AND e.id=$3 AND e.account_generation=$2 AND e.status IN('funding_suspended','disabled','closed')`
		args = append(args, intent.EnrollmentID)
	} else if intent.Purge {
		query += ` AND EXISTS(SELECT 1 FROM xnode_periods p WHERE p.id=e.period_id AND p.status<>'closed')`
	} else {
		query += ` AND false` // Resume needs only the users-row/paid-benefit fence.
	}
	rows, err := tx.QueryContext(ctx, query+` ORDER BY e.id LIMIT 128 FOR UPDATE`, args...)
	if err != nil {
		return nil, false, err
	}
	principals := []xnode.HourlyPrincipal{}
	for rows.Next() {
		p, e := scanHourlyPrincipal(rows)
		if e != nil {
			rows.Close()
			return nil, false, e
		}
		principals = append(principals, p)
	}
	if err = finishCompileRows(rows); err != nil {
		return nil, false, err
	}
	if !intent.Purge && !intent.Resume && len(principals) == 0 {
		return nil, false, xnode.ErrConflict
	}
	if intent.Purge && allowed {
		if _, err = tx.ExecContext(ctx, `UPDATE hourly_enrollments SET status='closed',updated_at=$3 WHERE owner_id=$1 AND account_generation<=$2 AND status<>'closed'`, intent.OwnerID, intent.ThroughGeneration, time.Now().UTC()); err != nil {
			return nil, false, err
		}
	}
	if allowed && mark != nil {
		if err = mark(); err != nil {
			return nil, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	if intent.Purge && !allowed {
		return nil, false, nil // A superseded purge must not close captured new state.
	}
	return principals, allowed, nil
}
