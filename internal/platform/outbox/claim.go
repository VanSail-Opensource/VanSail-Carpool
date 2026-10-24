package outbox

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CompleteClaim uses durable fencing when supported, retaining legacy adapters.
func CompleteClaim(ctx context.Context, repository interface {
	CompleteOutboxJob(context.Context, string, int, error, time.Time) error
}, job model.OutboxJob, jobErr error, now time.Time) error {
	if fenced, ok := repository.(interface {
		CompleteOutboxClaim(context.Context, model.OutboxJob, error, time.Time) error
	}); ok {
		return fenced.CompleteOutboxClaim(ctx, job, jobErr, now)
	}
	return repository.CompleteOutboxJob(ctx, job.ID, job.Attempts, jobErr, now)
}

// RenewClaim never lets a recovered attempt inherit an older worker heartbeat.
func RenewClaim(ctx context.Context, repository interface {
	RenewOutboxJob(context.Context, string, int, time.Time) (bool, error)
}, job model.OutboxJob, now time.Time) (bool, error) {
	if fenced, ok := repository.(interface {
		RenewOutboxClaim(context.Context, model.OutboxJob, time.Time) (bool, error)
	}); ok {
		return fenced.RenewOutboxClaim(ctx, job, now)
	}
	return repository.RenewOutboxJob(ctx, job.ID, job.Attempts, now)
}
