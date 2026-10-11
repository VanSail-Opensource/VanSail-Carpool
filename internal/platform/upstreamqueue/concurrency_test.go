package upstreamqueue

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestConcurrentWorkersPreserveGlobalPacingAndBound(t *testing.T) {
	queue := startTestQueue(t, Config{Name: "parallel", Capacity: 4, Workers: 2, MinInterval: 25 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan time.Time, 3)
	release := make(chan struct{})
	var calls sync.WaitGroup
	for i := 0; i < 3; i++ {
		calls.Add(1)
		go func() {
			defer calls.Done()
			if err := Execute(ctx, queue, func(ctx context.Context) error {
				started <- time.Now()
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	read := func() time.Time {
		select {
		case at := <-started:
			return at
		case <-ctx.Done():
			t.Fatal("worker did not start")
			return time.Time{}
		}
	}
	first, second := read(), read()
	if second.Sub(first) < 20*time.Millisecond {
		t.Fatal("per-worker pacing multiplied the provider rate")
	}
	select {
	case <-started:
		t.Fatal("exceeded two concurrent workers")
	case <-time.After(35 * time.Millisecond):
	}
	close(release)
	read()
	calls.Wait()
}

func TestConcurrentShutdownCancelsEveryWorker(t *testing.T) {
	q := startTestQueue(t, Config{Name: "cancel", Capacity: 2, Workers: 2})
	started := make(chan struct{}, 2)
	var calls sync.WaitGroup
	for i := 0; i < 2; i++ {
		calls.Add(1)
		go func() {
			defer calls.Done()
			_ = Execute(context.Background(), q, func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() })
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers serialized")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := q.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	calls.Wait()
}
