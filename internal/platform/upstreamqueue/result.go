package upstreamqueue

import (
	"context"
	"errors"
	"fmt"
)

type result[T any] struct {
	value T
	err   error
}

// Do submits fn and waits for its typed result. Caller cancellation applies
// while waiting for capacity, pacing, and provider execution.
func Do[T any](ctx context.Context, queue *Queue, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		return zero, errors.New("upstream call context is nil")
	}
	if queue == nil {
		return zero, errors.New("upstream queue is nil")
	}
	if fn == nil {
		return zero, errors.New("upstream call function is nil")
	}
	completed := make(chan result[T], 1)
	item := call{
		ctx: ctx,
		key: orderingKey(ctx),
		run: func(callCtx context.Context) {
			value, err := fn(callCtx)
			completed <- result[T]{value: value, err: err}
		},
		reject: func(err error) {
			completed <- result[T]{err: err}
		},
	}
	lifecycle, err := queue.enqueue(ctx, item)
	if err != nil {
		return zero, err
	}
	select {
	case outcome := <-completed:
		return outcome.value, outcome.err
	default:
	}
	select {
	case outcome := <-completed:
		return outcome.value, outcome.err
	case <-ctx.Done():
		select {
		case outcome := <-completed:
			return outcome.value, outcome.err
		default:
			return zero, fmt.Errorf("wait for %s queue: %w", queue.Name(), ctx.Err())
		}
	case <-lifecycle.Done():
		select {
		case outcome := <-completed:
			return outcome.value, outcome.err
		default:
			return zero, fmt.Errorf("wait for %s queue: %w", queue.Name(), lifecycle.Err())
		}
	}
}

// Execute submits an error-only provider call.
func Execute(ctx context.Context, queue *Queue, fn func(context.Context) error) error {
	_, err := Do(ctx, queue, func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, fn(callCtx)
	})
	return err
}
