package outbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type leaseRepository interface {
	RenewOutboxJob(context.Context, string, int, time.Time) (bool, error)
}

type executor struct {
	worker         *Worker
	ctx            context.Context
	cancel         context.CancelFunc
	report         func(error)
	mu             sync.Mutex
	stopping       bool
	active         sync.WaitGroup
	normal, urgent chan struct{}
	compute        chan struct{}
}

// Start enables four ordinary attempts and one reserved urgent attempt. The
// repository must heartbeat and fence leases; handlers must honor cancellation.
func (w *Worker) Start(ctx context.Context, report func(error)) error {
	if ctx == nil {
		return errors.New("outbox context is nil")
	}
	if _, ok := w.repository.(leaseRepository); !ok {
		return errors.New("outbox repository cannot renew leases")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.executor != nil {
		return errors.New("outbox executor already started")
	}
	ctx, cancel := context.WithCancel(ctx)
	w.executor = &executor{worker: w, ctx: ctx, cancel: cancel, report: report, normal: make(chan struct{}, 4), urgent: make(chan struct{}, 1), compute: make(chan struct{}, 16)}
	return nil
}

// Shutdown cancels and joins attempts before the database and providers close.
func (w *Worker) Shutdown() {
	w.mu.RLock()
	e := w.executor
	w.mu.RUnlock()
	if e == nil {
		return
	}
	e.mu.Lock()
	e.stopping = true
	e.cancel()
	e.mu.Unlock()
	e.active.Wait()
}

func (e *executor) drain(ctx context.Context, limit int, claim func(context.Context, time.Time) (*model.OutboxJob, error), priority bool, computeOnly ...bool) error {
	if limit <= 0 {
		limit = 20
	}
	slots := e.normal
	if priority {
		slots = e.urgent
	}
	if len(computeOnly) > 0 && computeOnly[0] {
		slots = e.compute
	}
	for range limit {
		e.mu.Lock()
		if e.stopping || e.ctx.Err() != nil {
			e.mu.Unlock()
			return context.Canceled
		}
		select {
		case slots <- struct{}{}:
		default:
			e.mu.Unlock()
			return nil
		}
		// Serialize reservation with shutdown; claiming is a short SQLite operation.
		job, err := claim(ctx, e.worker.now().UTC())
		if err != nil || job == nil {
			<-slots
			e.mu.Unlock()
			return err
		}
		e.active.Add(1)
		go func() { defer e.active.Done(); defer func() { <-slots }(); e.execute(*job) }()
		e.mu.Unlock()
	}
	return nil
}

func (e *executor) execute(job model.OutboxJob) {
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Minute)
	defer cancel()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewalCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				owned, err := RenewClaim(renewalCtx, e.worker.repository.(leaseRepository), job, e.worker.now().UTC())
				stop()
				if err != nil || !owned {
					cancel()
					return
				}
			}
		}
	}()
	jobErr := e.attempt(ctx, job)
	cancel()
	<-heartbeatDone
	if errors.Is(jobErr, ErrDeferred) {
		return
	} // Native executor owns its own heartbeat.
	completionCtx, stop := context.WithTimeout(context.WithoutCancel(e.ctx), 5*time.Second)
	defer stop()
	if err := CompleteClaim(completionCtx, e.worker.repository, job, jobErr, e.worker.now().UTC()); err != nil && e.report != nil {
		e.report(fmt.Errorf("complete outbox attempt %s: %w", job.ID, err))
	}
}

func (e *executor) attempt(ctx context.Context, job model.OutboxJob) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("outbox handler panicked")
		}
	}()
	return e.worker.dispatch(ctx, job)
}
