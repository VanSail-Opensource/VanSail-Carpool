package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type delayedPasswordClient struct {
	embyClient
	started, release chan struct{}
	current, next    []byte
	observedCurrent  string
	observedNext     string
}

func (c *delayedPasswordClient) SetPassword(_ context.Context, _ string, current, next []byte) error {
	c.current, c.next = current, next
	close(c.started)
	<-c.release // Model a transport that is still unwinding after cancellation.
	c.observedCurrent, c.observedNext = string(current), string(next)
	return nil
}

func TestEmbyPasswordCancellationKeepsExecutorBuffersUntilItStops(t *testing.T) {
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "emby-password-test", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	client := &delayedPasswordClient{started: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	t.Cleanup(func() {
		release.Do(func() { close(client.release) })
		if err := queue.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	adapter := embyAdapter{queue: queue, clientFactory: func(context.Context) (embyClient, error) { return client, nil }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	current, next := []byte("old-password"), []byte("new-password")
	done := make(chan error, 1)
	go func() { done <- adapter.SetPassword(ctx, "member", current, next) }()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("password callback did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled call: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled queued call did not return")
	}
	// Callers may retire their buffers immediately after the adapter returns.
	clear(current)
	clear(next)
	release.Do(func() { close(client.release) })
	if err := queue.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.observedCurrent != "old-password" || client.observedNext != "new-password" {
		t.Fatal("cancellation cleared secrets while the executor still needed them")
	}
	if strings.Trim(string(client.current), "\x00") != "" || strings.Trim(string(client.next), "\x00") != "" {
		t.Fatal("executor retained password buffers after completion")
	}
}
