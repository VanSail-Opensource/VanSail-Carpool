package browserauth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

func TestOAuthHTTPUsesExistingQueue(t *testing.T) {
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "telegram-oauth-test", Capacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	transport := &queuedTransport{queue: queue, transport: oauthTestTransport{run: func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("keys"))}, nil
	}}}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, telegramIssuer+"/.well-known/jwks.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = transport.RoundTrip(request); err == nil || calls.Load() != 0 {
		t.Fatal("HTTP bypassed stopped queue")
	}
	lifecycle, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = queue.Start(lifecycle); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := queue.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || string(data) != "keys" || calls.Load() != 1 {
		t.Fatalf("queued response=%q,calls=%d,error=%v", data, calls.Load(), err)
	}
}
