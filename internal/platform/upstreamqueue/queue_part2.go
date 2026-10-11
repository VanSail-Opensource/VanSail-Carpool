package upstreamqueue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func (q *Queue) run(lifecycle context.Context, done chan struct{}) {
	var active sync.WaitGroup
	defer func() {
		active.Wait()
		q.mu.Lock()
		q.state = stateStopped
		q.mu.Unlock()
		close(done)
	}()
	var nextStart time.Time
	slots := make(chan struct{}, q.workers)
	for {
		select {
		case slots <- struct{}{}:
		case <-lifecycle.Done():
			return
		}
		select {
		case <-lifecycle.Done():
			return
		case item := <-q.calls:
			if err := waitUntil(item.ctx, lifecycle, nextStart); err != nil {
				item.reject(err)
				<-slots
				continue
			}
			if err := item.ctx.Err(); err != nil {
				item.reject(err)
				<-slots
				continue
			}
			// One scheduler owns pacing across every concurrent worker.
			started := make(chan time.Time, 1)
			active.Add(1)
			go func() {
				defer active.Done()
				defer func() { <-slots }()
				q.execute(lifecycle, item, started)
			}()
			if at, ok := <-started; ok {
				nextStart = at.Add(q.minInterval)
			}
		}
	}
}

func (q *Queue) execute(lifecycle context.Context, item call, started chan<- time.Time) {
	defer close(started)
	if err := lifecycle.Err(); err != nil {
		item.reject(err)
		return
	}
	if err := item.ctx.Err(); err != nil {
		item.reject(err)
		return
	}
	executionCtx, cancel := mergeCancellation(item.ctx, lifecycle)
	defer cancel()
	defer func() {
		if recover() != nil {
			item.reject(fmt.Errorf("%s: %w", q.name, ErrTaskPanic))
		}
	}()
	started <- time.Now()
	item.run(executionCtx)
}

func waitUntil(caller, lifecycle context.Context, target time.Time) error {
	delay := time.Until(target)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-caller.Done():
		return caller.Err()
	case <-lifecycle.Done():
		return lifecycle.Err()
	}
}

func mergeCancellation(caller, lifecycle context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(caller)
	stop := context.AfterFunc(lifecycle, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}
