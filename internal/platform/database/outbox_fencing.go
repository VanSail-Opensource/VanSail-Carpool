package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CompleteOutboxJob preserves the legacy SQLite repository contract. PostgreSQL
// callers must supply the durable generation through CompleteOutboxClaim.
func (s *Store) CompleteOutboxJob(ctx context.Context, id string, attempts int, jobErr error, now time.Time) error {
	if IsPostgres(s.db) {
		return fmt.Errorf("PostgreSQL outbox completion requires a claim generation")
	}
	return s.completeOutboxClaim(ctx, id, attempts, 0, jobErr, now)
}

// CompleteOutboxClaim fences asynchronous completion across recovery/retry resets.
func (s *Store) CompleteOutboxClaim(ctx context.Context, job model.OutboxJob, jobErr error, now time.Time) error {
	return s.completeOutboxClaim(ctx, job.ID, job.Attempts, job.ClaimGeneration, jobErr, now)
}

// RenewOutboxClaim renews only the exact processing generation.
func (s *Store) RenewOutboxClaim(ctx context.Context, job model.OutboxJob, now time.Time) (bool, error) {
	if !IsPostgres(s.db) {
		return s.RenewOutboxJob(ctx, job.ID, job.Attempts, now)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET updated_at=? WHERE id=? AND status='processing' AND attempts=? AND claim_generation=?`, stamp(now), job.ID, job.Attempts, job.ClaimGeneration)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
