package connectivity

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestHostRetriesCancelDuringProbeOrDelay(t *testing.T) {
	for _, during := range []string{"probe", "delay"} {
		for _, reason := range []string{"settings", "shutdown"} {
			t.Run(during+"_"+reason, func(t *testing.T) {
				cfg := DefaultConfig()
				cfg.ProxyUserID, cfg.RetryIntervalSeconds = 42, 60
				entered, done := make(chan struct{}), make(chan struct{})
				var calls atomic.Int32
				probe := testProbe(func(ctx context.Context, _ Target, _ Config) Outcome {
					calls.Add(1)
					if during == "probe" {
						close(entered)
						<-ctx.Done()
						return Outcome{Status: "connected"}
					}
					return Outcome{Status: "failed", ErrorCode: "CONNECTIVITY_PROBE_FAILED"}
				})
				service, repository, item := retryFixture(t, cfg, probe)
				if during == "delay" {
					service.retryWait = func(ctx context.Context, delay time.Duration) error {
						close(entered)
						return waitForRetry(ctx, delay)
					}
				}
				go func() { service.process(item); close(done) }()
				waitSignal(t, entered)
				if reason == "settings" {
					service.Invalidate()
				} else {
					item.cancel()
				}
				waitSignal(t, done)
				items := repository.attempts()
				if calls.Load() != 1 || len(items) != 1 || items[0].Status != "interrupted" ||
					items[0].ErrorCode != "CONNECTIVITY_INTERRUPTED" || items[0].FinishedAt == nil || item.run.Status != "interrupted" {
					t.Fatalf("cancelled retry published a result: calls=%d run=%+v attempts=%+v", calls.Load(), item.run, items)
				}
			})
		}
	}
}

func TestRetryWaitHonorsDelayAndCancellation(t *testing.T) {
	started := time.Now()
	if err := waitForRetry(context.Background(), 10*time.Millisecond); err != nil || time.Since(started) < 10*time.Millisecond {
		t.Fatalf("retry delay ended early: %s %v", time.Since(started), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started = time.Now()
	if err := waitForRetry(ctx, time.Minute); !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
		t.Fatalf("retry wait ignored cancellation: %s %v", time.Since(started), err)
	}
}

func TestHostCheckBudgetIncludesEveryRetry(t *testing.T) {
	for _, test := range []struct {
		name                    string
		retries, timeout, delay int
		want                    time.Duration
	}{
		{name: "default", retries: 10, timeout: 15, delay: 1, want: 175 * time.Second},
		{name: "disabled", retries: 0, timeout: 15, delay: 60, want: 15 * time.Second},
		{name: "maximum", retries: 10, timeout: 60, delay: 60, want: 1260 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := Config{MaxRetries: test.retries, TimeoutSeconds: test.timeout, RetryIntervalSeconds: test.delay}
			if got := cfg.hostCheckBudget(); got != test.want {
				t.Fatalf("host budget=%s, want %s", got, test.want)
			}
		})
	}
}
