package iplookup

import (
	"context"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

// The queue can return cancellation before its callback exits. Join entered
// callbacks and reject late starts before releasing the diagnostic admission slot.
func (s *EnhancedService) runSourceTest(b *sourceTestRuntime, test ProxySourceTest, source ProxySource, c Config) {
	defer b.work.Done()
	callCtx, cancel := context.WithTimeout(b.ctx, 20*time.Minute)
	defer cancel()
	finished := make(chan struct{})
	entered, abandoned := false, false // Both flags are protected by b.mu.
	runErr := upstreamqueue.Execute(callCtx, s.queues["checks"], func(ctx context.Context) error {
		b.mu.Lock()
		if abandoned {
			b.mu.Unlock()
			return context.Canceled
		}
		entered = true
		b.mu.Unlock()
		defer close(finished)
		update := func(result ProxySourceResult) {
			b.mu.Lock()
			b.entries[test.ID] = sourceTestEntry{test: ProxySourceTest{ID: test.ID, Status: "processing", Result: result}}
			b.mu.Unlock()
		}
		_, result := s.collectSource(ctx, source, c, true, update)
		if ctx.Err() != nil {
			result.Status = "partial"
			result.ErrorCode = "IP_PROXY_INTERRUPTED"
		}
		b.mu.Lock()
		b.entries[test.ID] = sourceTestEntry{test: ProxySourceTest{ID: test.ID, Status: "completed", Result: result}, expires: time.Now().Add(15 * time.Minute)}
		b.mu.Unlock()
		return ctx.Err()
	})
	b.mu.Lock()
	wait := entered
	if !wait {
		abandoned = true
	}
	b.mu.Unlock()
	if wait {
		<-finished
	}
	b.mu.Lock()
	entry := b.entries[test.ID]
	if runErr != nil && entry.test.Status != "completed" {
		entry.test.Status = "completed"
		entry.test.Result.Status = "partial"
		entry.test.Result.ErrorCode = "IP_PROXY_INTERRUPTED"
		entry.expires = time.Now().Add(15 * time.Minute)
		b.entries[test.ID] = entry
	}
	b.active = false
	b.mu.Unlock()
}
