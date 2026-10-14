package turnstile

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSiteverifyQueueIdentityAndFailClosedResults(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"valid", `{"success":true,"hostname":"panel.example","action":"txc_first_entry"}`, 200, nil},
		{"wrong hostname", `{"success":true,"hostname":"other.example","action":"txc_first_entry"}`, 200, ErrRejected},
		{"wrong action", `{"success":true,"hostname":"panel.example","action":"login"}`, 200, ErrRejected},
		{"expired or replayed", `{"success":false,"error-codes":["timeout-or-duplicate"]}`, 200, ErrRejected},
		{"provider error", `{"success":false,"error-codes":["internal-error"]}`, 200, ErrUnavailable},
		{"bad secret", `{"success":false,"error-codes":["invalid-input-secret"]}`, 200, ErrUnavailable},
		{"malformed", `{`, 200, ErrUnavailable},
		{"HTTP failure", `{}`, 503, ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "turnstile-test", Capacity: 2})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := queue.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = queue.Shutdown(context.Background()) }()
			verifier := New(queue)
			calls := 0
			verifier.Client.Transport = transportFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.String() != "https://challenges.cloudflare.com/turnstile/v0/siteverify" || request.Method != "POST" {
					t.Errorf("request = %s %s", request.Method, request.URL)
				}
				if err := request.ParseForm(); err != nil {
					t.Error(err)
					return nil, err
				}
				if request.Form.Get("secret") != "server-secret" || request.Form.Get("response") != "single-use-token" || request.Form.Get("idempotency_key") == "" {
					t.Error("incomplete Siteverify form")
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})
			if err := verifier.Verify(ctx, "server-secret", "single-use-token", "panel.example"); !errors.Is(err, test.want) || calls != 1 {
				t.Fatalf("Verify=%v calls=%d", err, calls)
			}
		})
	}
}

func TestSiteverifyNeverExecutesOutsideQueueOrWithInvalidToken(t *testing.T) {
	queue, _ := upstreamqueue.New(upstreamqueue.Config{Name: "not-started", Capacity: 1})
	verifier := New(queue)
	if err := verifier.Verify(context.Background(), "secret", "token", "panel.example"); !errors.Is(err, upstreamqueue.ErrNotRunning) {
		t.Fatalf("queue bypass: %v", err)
	}
	if err := verifier.Verify(context.Background(), "secret", strings.Repeat("x", 2049), "panel.example"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unbounded token: %v", err)
	}
}
