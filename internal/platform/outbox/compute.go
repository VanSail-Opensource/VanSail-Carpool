package outbox

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type computeRepository interface {
	ClaimComputeOutboxJob(context.Context, time.Time) (*model.OutboxJob, error)
}

// DrainCompute uses the same durable leases and receipts with independent
// bounded slots. Waiting for a result cannot occupy an integration worker.
func (w *Worker) DrainCompute(ctx context.Context, limit int) error {
	r, ok := w.repository.(computeRepository)
	if !ok {
		return nil
	}
	w.mu.RLock()
	e := w.executor
	w.mu.RUnlock()
	if e != nil {
		return e.drain(ctx, limit, r.ClaimComputeOutboxJob, false, true)
	}
	return w.drain(ctx, limit, r.ClaimComputeOutboxJob, false)
}
