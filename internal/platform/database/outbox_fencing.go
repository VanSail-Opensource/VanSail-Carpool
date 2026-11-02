package database

import (
	"context"

	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CompleteOutboxClaim fences asynchronous completion across recovery/retry resets.
func (s *Store) CompleteOutboxClaim(ctx context.Context, job model.OutboxJob, jobErr error, now time.Time) error {
	return s.completeOutboxClaim(ctx, job.ID, job.Attempts, job.ClaimGeneration, jobErr, now)
}

// RenewOutboxClaim renews only the exact processing generation.
func (s *Store) RenewOutboxClaim(ctx context.Context, job model.OutboxJob, now time.Time) (bool, error) {

	result, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET updated_at=$1 WHERE id=$2 AND status='processing' AND attempts=$3 AND claim_generation=$4`, stamp(now), job.ID, job.Attempts, job.ClaimGeneration)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
