package upstreamqueue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func (q *Queue) run(lifecycle context.Context, done chan struct{}) {
	var active sync.WaitGroup
	pending := []call{}
	finished := make(chan string, q.workers)
	occupied := map[string]bool{}
	running := 0
	defer func() {
		for _, item := range pending {
			item.reject(lifecycle.Err())
		}
		active.Wait()
		q.mu.Lock()
		q.state = stateStopped
		q.mu.Unlock()
		close(done)
	}()
	var nextStart time.Time
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		if lifecycle.Err() != nil {
			return
		}
		ready := -1
		for i := 0; i < len(pending); {
			if err := pending[i].ctx.Err(); err != nil {
				pending[i].reject(err)
				pending = append(pending[:i], pending[i+1:]...)
				continue
			}
			if ready < 0 && (pending[i].key == "" || !occupied[pending[i].key]) {
				ready = i
			}
			i++
		}
		if running < q.workers && ready >= 0 && !time.Now().Before(nextStart) {
			item := pending[ready]
			pending = append(pending[:ready], pending[ready+1:]...)
			if item.key != "" {
				occupied[item.key] = true
			}
			running++
			started := make(chan time.Time, 1)
			active.Add(1)
			go func() {
				defer active.Done()
				defer func() { finished <- item.key }()
				q.execute(lifecycle, item, started)
			}()
			if at, ok := <-started; ok {
				nextStart = at.Add(q.minInterval)
			}
			continue
		}
		var incoming <-chan call
		if len(pending) < q.capacity {
			incoming = q.calls
		}
		var paced <-chan time.Time
		if running < q.workers && ready >= 0 {
			timer.Reset(max(time.Nanosecond, time.Until(nextStart)))
			paced = timer.C
		}
		select {
		case <-lifecycle.Done():
			return
		case key := <-finished:
			running--
			delete(occupied, key)
		case item := <-incoming:
			pending = append(pending, item)
		case <-paced:
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

func mergeCancellation(caller, lifecycle context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(caller)
	stop := context.AfterFunc(lifecycle, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}
