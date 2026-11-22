package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Retry only the fenced persistence step, never the handler. Completion can
// safely be repeated after an ambiguous commit: a missing row or a different
// claim generation is a no-op. Keep a short detached budget during shutdown so
// a transient database failure does not strand completed work until recovery.
func (e *executor) complete(job model.OutboxJob, jobErr error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(e.ctx), 5*time.Second)
	defer cancel()
	for attempt := range 3 {
		err := CompleteClaim(ctx, e.worker.repository, job, jobErr, e.worker.now().UTC())
		if err == nil || attempt == 2 {
			return err
		}
		timer := time.NewTimer(100 * time.Millisecond << attempt)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}
	return nil
}
