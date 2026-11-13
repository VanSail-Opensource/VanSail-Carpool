package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// The callback may only change bounded in-memory admission state. It runs under
// the same users-row lock as activation/renewal/resume and performs no disk or
// network I/O. Native period settlement happens after this transaction returns.
func (s *Store) WithHourlyOwnerLifecycle(ctx context.Context, intent xnode.HourlyLifecycleIntent, mark func() error) ([]xnode.HourlyPrincipal, bool, error) {
	// A purge additionally needs final reachability admission and the durable
	// mixed-period cursor. It must use AccountNativePurgePage/Advance instead.
	if intent.Purge || intent.OwnerID == "" || intent.CurrentGeneration < 0 || intent.ThroughGeneration < 0 || intent.ThroughGeneration > intent.CurrentGeneration || !intent.Resume && intent.EnrollmentID == "" {
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
	if intent.Resume {
		if current == intent.CurrentGeneration && status == "active" {
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2 AND status='active' AND paid_through>clock_timestamp())`, intent.OwnerID, current).Scan(&allowed)
		}
	} else {
		var newer bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2 AND (status='preparing' OR status='active' AND paid_through>clock_timestamp())) OR EXISTS(SELECT 1 FROM purchases WHERE user_id=$1 AND status IN('active','activating','queued') AND `+purchaseTimestampOrderSQL("valid_until")+`>`+databaseClockTimestampOrderSQL+`)`, intent.OwnerID, current).Scan(&newer)
		allowed = current == intent.CurrentGeneration && status == "active" && !newer
	}
	if err != nil {
		return nil, false, err
	}
	query := `SELECT ` + hourlyPrincipalColumns + ` FROM hourly_enrollments e WHERE e.owner_id=$1 AND e.account_generation<=$2`
	args := []any{intent.OwnerID, intent.ThroughGeneration}
	if !intent.Resume {
		query += ` AND e.id=$3 AND e.account_generation=$2 AND e.status IN('funding_suspended','disabled','closed')`
		args = append(args, intent.EnrollmentID)
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
	if !intent.Resume && len(principals) == 0 {
		return nil, false, xnode.ErrConflict
	}
	if allowed && mark != nil {
		if err = mark(); err != nil {
			return nil, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return principals, allowed, nil
}
