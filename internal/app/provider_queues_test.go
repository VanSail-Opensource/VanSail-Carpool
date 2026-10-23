package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func TestProviderQueueConfiguration(t *testing.T) {
	queues, err := newProviderQueues()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		queue        *upstreamqueue.Queue
		wantCapacity int
		wantPace     time.Duration
	}{
		{name: "Emby", queue: queues.emby, wantCapacity: embyQueueCapacity, wantPace: embyPace},
		{name: "Telegram", queue: queues.telegram, wantCapacity: telegramQueueCapacity, wantPace: telegramPace},
		{name: "Turnstile", queue: queues.turnstile, wantCapacity: 32, wantPace: 100 * time.Millisecond},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.queue.Name() == "" || test.queue.Capacity() != test.wantCapacity || test.queue.MinInterval() != test.wantPace {
				t.Fatalf("queue config = (%q, %d, %s)", test.queue.Name(), test.queue.Capacity(), test.queue.MinInterval())
			}
		})
	}
}

func TestProviderQueuesUseIndependentWorkers(t *testing.T) {
	queues, err := newProviderQueues()
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := queues.start(lifecycle); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := queues.shutdown(context.Background()); err != nil {
			t.Errorf("shutdown queues: %v", err)
		}
	})

	telegramStarted := make(chan struct{})
	telegramDone := make(chan error, 1)
	go func() {
		telegramDone <- upstreamqueue.Execute(context.Background(), queues.telegram, func(ctx context.Context) error {
			close(telegramStarted)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	<-telegramStarted

	embyDone := make(chan error, 1)
	go func() {
		embyDone <- upstreamqueue.Execute(context.Background(), queues.emby, func(context.Context) error { return nil })
	}()
	select {
	case err := <-embyDone:
		if err != nil {
			t.Fatalf("Emby call failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Emby worker was blocked by Telegram")
	}

	cancel()
	if err := <-telegramDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("Telegram call error = %v, want canceled", err)
	}
}
