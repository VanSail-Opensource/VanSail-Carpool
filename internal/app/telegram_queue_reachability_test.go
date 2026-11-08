package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func TestReachabilityAdapterCannotBypassStoppedUpstreamQueue(t *testing.T) {
	q, err := upstreamqueue.New(upstreamqueue.Config{Name: "cleanup-fixture", Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &queuedTelegram{queue: q}
	if _, err = adapter.SendChatAction(context.Background(), 42); !errors.Is(err, upstreamqueue.ErrNotRunning) {
		t.Fatalf("action bypassed queue: %v", err)
	}
	if _, err = adapter.GetChat(context.Background(), 42); !errors.Is(err, upstreamqueue.ErrNotRunning) {
		t.Fatalf("chat read bypassed queue: %v", err)
	}
}

type cleanupQueueTransport struct {
	started, release chan struct{}
	calls            atomic.Int32
}

func (d *cleanupQueueTransport) Do(request *http.Request) (*http.Response, error) {
	d.calls.Add(1)
	result := `{"ok":true,"result":{"id":123,"is_bot":true,"username":"fixture"}}`
	if strings.HasSuffix(request.URL.Path, "/sendChatAction") {
		close(d.started)
		select {
		case <-d.release:
			result = `{"ok":true,"result":true}`
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(result)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
}

func TestSlowCleanupActionLeavesOtherExistingTelegramWorkersAvailable(t *testing.T) {
	q, err := upstreamqueue.New(upstreamqueue.Config{Name: "cleanup-fairness", Capacity: 4, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdown, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		if err := q.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
	})
	doer := &cleanupQueueTransport{started: make(chan struct{}), release: make(chan struct{})}
	client, err := telegram.NewClient("123:fixture", telegram.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	adapter := &queuedTelegram{queue: q, client: client}
	done := make(chan error, 1)
	go func() { _, err := adapter.SendChatAction(ctx, 42); done <- err }()
	select {
	case <-doer.started:
	case <-time.After(time.Second):
		t.Fatal("cleanup action never entered provider queue")
	}
	other, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	if bot, err := adapter.GetMe(other); err != nil || bot.ID != 123 {
		t.Fatalf("unrelated queued call was blocked: %v", err)
	}
	close(doer.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup action did not release")
	}
	if doer.calls.Load() != 2 {
		t.Fatal("provider request unexpectedly duplicated")
	}
}
