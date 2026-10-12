package iplookup

import (
	"context"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

type backgroundRepository interface {
	PendingIPLookupOperations(context.Context) ([]string, error)
	ProviderOperationByID(context.Context, string) (providerops.Operation, error)
	ClaimIPLookupLease(context.Context, string, string, time.Time) (bool, error)
	HeartbeatIPLookup(context.Context, string, string, time.Time) error
}
type backgroundRuntime struct {
	ctx      context.Context
	queue    *upstreamqueue.Queue
	mu       sync.Mutex
	active   map[string]bool
	work     sync.WaitGroup
	done     chan struct{}
	stopping bool
	owner    string
}

// StartBackground attaches durable lookups to the application, never a browser request.
func (w *Worker) StartBackground(ctx context.Context, queue *upstreamqueue.Queue) {
	if w.Enhanced != nil {
		w.Enhanced.StartSourceTests(ctx)
	}
	owner, err := ids.New()
	if err != nil {
		if w.OnBackgroundError != nil {
			w.OnBackgroundError("IP_LOOKUP_FAILED")
		}
		return
	}
	b := &backgroundRuntime{ctx: ctx, queue: queue, active: map[string]bool{}, done: make(chan struct{}), owner: owner}
	w.background = b
	go func() {
		defer close(b.done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			if repo, ok := w.repository.(backgroundRepository); ok {
				if ids, err := repo.PendingIPLookupOperations(ctx); err == nil {
					for _, id := range ids {
						if err := w.launchBackground(id); err != nil {
							break
						}
					}
				}
			}
			select {
			case <-ctx.Done():
				b.mu.Lock()
				b.stopping = true
				b.mu.Unlock()
				b.work.Wait()
				return
			case <-ticker.C:
			}
		}
	}()
}

// WaitBackground joins all database-dependent work before SQLite closes.
func (w *Worker) WaitBackground() {
	if w.background != nil {
		<-w.background.done
	}
	if w.Enhanced != nil {
		w.Enhanced.WaitSourceTests()
	}
}

func (w *Worker) launchBackground(id string) error {
	b := w.background
	if err := b.ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	if b.stopping || b.ctx.Err() != nil {
		b.mu.Unlock()
		return context.Canceled
	}
	if b.active[id] {
		b.mu.Unlock()
		return nil
	}
	if len(b.active) >= 1 {
		b.mu.Unlock()
		// The report itself is the durable backlog; admission is retried by the scan.
		return nil
	}
	b.active[id] = true
	b.work.Add(1)
	b.mu.Unlock()
	go func() {
		defer b.work.Done()
		defer func() { b.mu.Lock(); delete(b.active, id); b.mu.Unlock() }()
		repo, ok := w.repository.(backgroundRepository)
		if !ok {
			return
		}
		// Failed executions retain their checkpoint; the recovery scan retries them.
		err := upstreamqueue.Execute(b.ctx, b.queue, func(ctx context.Context) error {
			return w.executeBackground(ctx, repo, id)
		})
		if err != nil && b.ctx.Err() == nil && w.OnBackgroundError != nil {
			w.OnBackgroundError(ErrorCode(err))
		}
	}()
	return nil
}
