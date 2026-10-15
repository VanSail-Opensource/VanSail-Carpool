package usdt

import (
	"context"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

// Executor shares one bounded queue and concurrency limit across every chain endpoint.
type Executor struct {
	mu    sync.RWMutex
	queue *upstreamqueue.Queue
}

func newExecutor() *Executor { return &Executor{} }

func (e *Executor) configure(ctx context.Context, concurrency int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.queue != nil {
		if err := e.queue.Shutdown(ctx); err != nil {
			return err
		}
		e.queue = nil
	}
	queue, err := upstreamqueue.New(upstreamqueue.Config{
		Name:     "usdt-rpc",
		Capacity: 32,
		Workers:  concurrency,
	})
	if err != nil {
		return err
	}
	if err := queue.Start(ctx); err != nil {
		return err
	}
	e.queue = queue
	return nil
}

func (e *Executor) close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.queue == nil {
		return nil
	}
	err := e.queue.Shutdown(ctx)
	e.queue = nil
	return err
}

func (e *Executor) do(ctx context.Context, call func(context.Context) error) error {
	e.mu.RLock()
	queue := e.queue
	e.mu.RUnlock()
	if queue == nil {
		return ErrUnavailable
	}
	var gate sync.Mutex
	started, stopped := false, false
	done := make(chan struct{})
	err := upstreamqueue.Execute(ctx, queue, func(jobCtx context.Context) error {
		gate.Lock()
		if stopped {
			gate.Unlock()
			return context.Canceled
		}
		started = true
		gate.Unlock()
		defer close(done)
		callCtx, cancel := context.WithTimeout(jobCtx, 15*time.Second)
		defer cancel()
		return call(callCtx)
	})
	gate.Lock()
	stopped = true
	join := started
	gate.Unlock()
	if join {
		<-done
	}
	return err
}
