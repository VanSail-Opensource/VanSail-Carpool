package outbox

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type priorityRepository interface {
	ClaimPriorityOutboxJob(context.Context, time.Time) (*model.OutboxJob, error)
}

// DrainPriority dispatches only urgent desired-state work using the existing
// outbox leases, handlers and completion path. It may run beside an ordinary
// drain that is waiting on an unrelated provider. Claims remain exclusive.
func (w *Worker) DrainPriority(ctx context.Context, limit int) error {
	repository, ok := w.repository.(priorityRepository)
	if !ok {
		return nil
	}
	return w.drain(ctx, limit, repository.ClaimPriorityOutboxJob, true)
}
