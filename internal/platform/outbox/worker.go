// Package outbox dispatches durable jobs to kind-specific handlers.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ErrUnknownKind indicates that a persisted job has no registered owner.
var ErrUnknownKind = errors.New("outbox job kind is not registered")

// ErrDeferred means a runtime-owned task retains this durable lease. It will
// complete the job itself; a process crash leaves it recoverable at startup.
var ErrDeferred = errors.New("outbox completion is deferred")

// ErrBusy did not attempt external work and must not exhaust delivery retries.
var ErrBusy = errors.New("outbox executor capacity is occupied")

// Repository leases and completes durable jobs.
type Repository interface {
	ClaimOutboxJob(context.Context, time.Time) (*model.OutboxJob, error)
	CompleteOutboxJob(context.Context, string, int, error, time.Time) error
}

// Handler performs one idempotent job attempt.
type Handler interface {
	HandleOutbox(context.Context, model.OutboxJob) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, model.OutboxJob) error

// HandleOutbox calls f.
func (f HandlerFunc) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	return f(ctx, job)
}

// Worker owns the single durable claim loop and routes each job by kind.
// Registration happens during application composition before Drain is called.
type Worker struct {
	repository Repository
	now        func() time.Time

	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewWorker creates an empty dispatcher.
func NewWorker(repository Repository) *Worker {
	return &Worker{repository: repository, now: time.Now, handlers: make(map[string]Handler)}
}

// Register assigns one unique kind to a handler.
func (w *Worker) Register(kind string, handler Handler) error {
	if kind == "" || handler == nil {
		return errors.New("outbox registration requires kind and handler")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.handlers[kind]; exists {
		return fmt.Errorf("register outbox kind %q: %w", kind, errors.New("duplicate registration"))
	}
	w.handlers[kind] = handler
	return nil
}

// Kinds returns the registered kinds in stable order for readiness diagnostics.
func (w *Worker) Kinds() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	kinds := make([]string, 0, len(w.handlers))
	for kind := range w.handlers {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// Drain processes up to limit due jobs serially. Serial dispatch preserves the
// existing SQLite write ordering while every handler remains independently
// retryable and context-cancellable.
func (w *Worker) Drain(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 20
	}
	for range limit {
		if err := ctx.Err(); err != nil {
			return err
		}
		job, err := w.repository.ClaimOutboxJob(ctx, w.now().UTC())
		if err != nil {
			return fmt.Errorf("claim outbox job: %w", err)
		}
		if job == nil {
			return nil
		}
		jobErr := w.dispatch(ctx, *job)
		if errors.Is(jobErr, ErrDeferred) {
			continue
		}
		if err := w.repository.CompleteOutboxJob(ctx, job.ID, job.Attempts, jobErr, w.now().UTC()); err != nil {
			return fmt.Errorf("complete outbox job %s: %w", job.ID, err)
		}
	}
	return nil
}

func (w *Worker) dispatch(ctx context.Context, job model.OutboxJob) error {
	w.mu.RLock()
	handler := w.handlers[job.Kind]
	w.mu.RUnlock()
	if handler == nil {
		return fmt.Errorf("%w: %s", ErrUnknownKind, job.Kind)
	}
	if err := handler.HandleOutbox(ctx, job); err != nil {
		return fmt.Errorf("handle outbox kind %s: %w", job.Kind, err)
	}
	return nil
}
