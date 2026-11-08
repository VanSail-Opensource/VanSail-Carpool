package database

import (
	"context"
	"database/sql"
	"errors"
)

// ErrHourlyCapacity preserves the bounded shared allocator's owner limit.
var ErrHourlyCapacity = errors.New("hourly account capacity is occupied")

const maxHourlyActiveOwners = 8192

type hourlyCapacityReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func hourlyOwnerCapacity(ctx context.Context, reader hourlyCapacityReader, owner string, generation int64, limit int) (bool, error) {
	var existing bool
	var count int
	err := reader.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments e WHERE e.owner_id=u.id
		AND e.account_generation=u.account_generation AND e.status='active'),(SELECT COUNT(*) FROM (
		SELECT e.owner_id FROM hourly_enrollments e JOIN users a ON a.id=e.owner_id
		AND a.account_generation=e.account_generation AND a.account_status='active' WHERE e.status='active'
		GROUP BY e.owner_id LIMIT $3) occupied) FROM users u
		WHERE u.id=$1 AND u.account_generation=$2 AND u.account_status='active'`, owner, generation, limit).Scan(&existing, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrAccountUnavailable
	}
	return existing || count < limit, err
}

// HourlyOwnerCapacityAvailable is advisory presentation only. Commit repeats
// the same check while holding the exclusive native compile revision row.
func (s *Store) HourlyOwnerCapacityAvailable(ctx context.Context, owner string, generation int64) (bool, error) {
	return hourlyOwnerCapacity(ctx, s.db, owner, generation, maxHourlyActiveOwners)
}

func requireHourlyOwnerCapacityTx(ctx context.Context, tx *sql.Tx, owner string, generation int64, limit int) error {
	available, err := hourlyOwnerCapacity(ctx, tx, owner, generation, limit)
	if err == nil && !available {
		err = ErrHourlyCapacity
	}
	return err
}
