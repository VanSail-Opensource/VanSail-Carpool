package upstreamqueue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledKeyedBacklogReleasesCapacityBeforeActiveCallFinishes(t *testing.T) {
	queue := startTestQueue(t, Config{Name: "cancel-backlog", Capacity: 1, Workers: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan struct{})
	activeDone := make(chan error, 1)
	go func() {
		activeDone <- Execute(WithKey(ctx, "held"), queue, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("active keyed call did not start")
	}

	waitingCtx, cancelWaiting := context.WithCancel(ctx)
	defer cancelWaiting()
	rejected := make(chan error, 1)
	if _, err := queue.enqueue(waitingCtx, call{
		ctx: waitingCtx, key: "held",
		run:    func(context.Context) { rejected <- errors.New("canceled keyed callback ran") },
		reject: func(err error) { rejected <- err },
	}); err != nil {
		t.Fatal("admit keyed backlog", err)
	}
	// The admission handshake guarantees that the sole pending slot is occupied.
	// Canceling it must wake the scheduler even while its active predecessor waits.
	cancelWaiting()
	select {
	case err := <-rejected:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("canceled backlog was executed", err)
		}
	case <-ctx.Done():
		t.Fatal("canceled backlog retained capacity until its predecessor completed")
	}
	if err := Execute(WithKey(ctx, "independent"), queue, func(context.Context) error { return nil }); err != nil {
		t.Fatal("idle worker could not admit independent work", err)
	}
	select {
	case err := <-activeDone:
		t.Fatal("active predecessor completed before independent work", err)
	default:
	}
	cancel()
	<-activeDone
}
