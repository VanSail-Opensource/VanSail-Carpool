package iplookup

import (
	"context"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

// WorkerRepository persists progress before each external request and atomically settles costs.
type WorkerRepository interface {
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	IPLookupRun(context.Context, string) (Report, Config, error)
	SaveIPLookupProgress(context.Context, Report) error
	FinishIPLookupRun(context.Context, Report, time.Time) error
}

// Provider is a queued adapter with a stable registry identity.
type Provider interface {
	ID() string
	Lookup(context.Context, string, ProviderConfig) (ProviderResult, error)
}

// Worker resumes report stages in the single existing durable operation lane.
type Worker struct {
	mu         sync.Mutex
	runs       map[string]*workerRun
	Live       *LiveService
	repository WorkerRepository
	providers  map[string]Provider
	now        func() time.Time
}

// NewWorker registers adapters without coupling verdict or billing to their HTTP schemas.
func NewWorker(repo WorkerRepository, providers []Provider) *Worker {
	w := &Worker{repository: repo, providers: map[string]Provider{}, now: time.Now}
	for _, p := range providers {
		w.providers[p.ID()] = p
	}
	return w
}

// HandleProviderOperation persists completed stages and does not retry interrupted HTTP calls.
func (w *Worker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	if operation.Receipt.Status == "queued" {
		if _, err := w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC()); err != nil {
			return err
		}
	}
	r, c, err := w.repository.IPLookupRun(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	unlock := w.lockRun(r.ID)
	defer unlock()
	r, c, err = w.repository.IPLookupRun(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if r.Status != "processing" {
		return w.repository.FinishIPLookupRun(ctx, r, w.now().UTC())
	}
	foldLegacy(&r, c)
	if r.Checkpoint == nil {
		return &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	if err := w.runQueuedStages(ctx, &r, c, func(progress Report) error {
		return w.repository.SaveIPLookupProgress(ctx, progress)
	}); err != nil {
		return err
	}
	if err := w.refreshLive(ctx, r, c); err != nil {
		return err
	}
	r = Aggregate(r, w.now().UTC())
	return w.repository.FinishIPLookupRun(ctx, r, w.now().UTC())
}

// runQueuedStages executes every pending stage in order. The persist callback
// receives the report after each durable transition; repair passes nil-free
// persistence against the frozen cache row instead of live progress.
func (w *Worker) runQueuedStages(ctx context.Context, r *Report, c Config, persist func(Report) error) error {
	for index := range r.Checkpoint.Stages {
		p := &r.Checkpoint.Stages[index]
		if p.Status == "disabled" || p.Status == "skipped" {
			continue
		}
		if len(r.Refusals) > 0 {
			skipLater(r, index)
			break
		}
		if p.Status == "success" || p.Status == "partial" || p.Status == "error" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		result := ProviderResult{ID: p.ID, Status: "error"}
		if p.Status == "processing" {
			// The persisted attempted marker forbids replay after an interrupted call.
			p.Attempted = true
		} else {
			p.Status, p.Attempted = "processing", true
			if persist != nil {
				if err := persist(*r); err != nil {
					return err
				}
			}
			adapter := w.providers[p.ID]
			if adapter != nil {
				configuration := ProviderConfig{ID: p.ID}
				for _, candidate := range c.Providers {
					if candidate.ID == p.ID {
						configuration = candidate
						break
					}
				}
				lookup, callErr := adapter.Lookup(ctx, r.IP, configuration)
				if callErr == nil {
					result = lookup
					result.ID = p.ID
				}
			}
		}
		RecordProvider(r, result, c)
		if persist != nil {
			if err := persist(*r); err != nil {
				return err
			}
		}
		if len(r.Refusals) > 0 {
			skipLater(r, index)
			break
		}
	}
	return nil
}

func skipLater(r *Report, index int) {
	for i := index + 1; i < len(r.Checkpoint.Stages); i++ {
		if r.Checkpoint.Stages[i].Status == "queued" {
			r.Checkpoint.Stages[i].Status = "skipped"
		}
	}
}
