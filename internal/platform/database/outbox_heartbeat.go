package database

import (
	"context"
	"time"
)

// RenewOutboxJob returns false when another attempt already owns the job.
func (s *Store) RenewOutboxJob(ctx context.Context, id string, attempt int, now time.Time) (bool, error) {
	if IsPostgres(s.db) {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET updated_at=? WHERE id=? AND attempts=? AND status='processing'`, stamp(now), id, attempt)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
