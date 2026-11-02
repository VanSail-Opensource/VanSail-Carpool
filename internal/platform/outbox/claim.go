package outbox

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

// CompleteClaim always supplies the durable claim generation.
func CompleteClaim(ctx context.Context, repository interface {
	CompleteOutboxClaim(context.Context, model.OutboxJob, error, time.Time) error
}, job model.OutboxJob, jobErr error, now time.Time) error {
	return repository.CompleteOutboxClaim(ctx, job, jobErr, now)
}

// RenewClaim cannot transfer a recovered lease to an older worker.
func RenewClaim(ctx context.Context, repository interface {
	RenewOutboxClaim(context.Context, model.OutboxJob, time.Time) (bool, error)
}, job model.OutboxJob, now time.Time) (bool, error) {
	return repository.RenewOutboxClaim(ctx, job, now)
}
