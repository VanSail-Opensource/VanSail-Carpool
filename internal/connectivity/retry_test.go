package connectivity

import (
	"context"
	"testing"
	"time"
)

func TestHostRetriesPublishOnlyFinalOutcome(t *testing.T) {
	for _, test := range []struct {
		name       string
		retries    int
		failures   int
		terminal   string
		wantCalls  int
		wantStatus string
	}{
		{"immediate_success", 10, 0, "connected", 1, "connected"},
		{"recovers", 10, 2, "connected", 3, "connected"},
		{"default_exhausted", 10, 11, "connected", 11, "failed"},
		{"zero_retries", 0, 1, "connected", 1, "failed"},
		{"unsupported", 10, 0, "unsupported", 1, "unsupported"},
		{"checker_error", 10, 0, "error", 1, "error"},
		{"interrupted", 10, 0, "interrupted", 1, "interrupted"},
		{"invalid_outcome", 10, 0, "invalid", 1, "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.ProxyUserID, cfg.MaxRetries = 42, test.retries
			calls, waits := 0, 0
			var repository *testRepository
			probe := testProbe(func(ctx context.Context, _ Target, _ Config) Outcome {
				calls++
				items := repository.attempts()
				if len(items) != 1 || items[0].Status != "running" || items[0].FinishedAt != nil {
					t.Errorf("intermediate result was published: %+v", items)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Error("queued attempt has no deadline")
				}
				if calls <= test.failures {
					status := 503
					return Outcome{Status: "failed", HTTPStatus: &status, ErrorCode: "CONNECTIVITY_UNEXPECTED_HTTP_STATUS"}
				}
				status, latency := 204, 12.5
				return Outcome{Status: test.terminal, HTTPStatus: &status, LatencyMS: &latency}
			})
			service, retained, item := retryFixture(t, cfg, probe)
			repository = retained
			service.retryWait = func(ctx context.Context, delay time.Duration) error {
				waits++
				if delay != time.Second {
					t.Errorf("retry delay=%s", delay)
				}
				return ctx.Err()
			}
			service.process(item)
			items := retained.attempts()
			if calls != test.wantCalls || waits != test.wantCalls-1 || len(items) != 1 ||
				items[0].Status != test.wantStatus || items[0].FinishedAt == nil || item.run.Completed != 1 {
				t.Fatalf("calls=%d waits=%d run=%+v attempts=%+v", calls, waits, item.run, items)
			}
			if test.wantStatus == "connected" && (items[0].LatencyMS == nil || *items[0].LatencyMS != 12.5 ||
				items[0].HTTPStatus == nil || *items[0].HTTPStatus != 204 || items[0].ErrorCode != "") {
				t.Fatalf("final successful diagnostics lost: %+v", items[0])
			}
			if test.wantStatus == "failed" && (items[0].HTTPStatus == nil || *items[0].HTTPStatus != 503 ||
				items[0].ErrorCode != "CONNECTIVITY_UNEXPECTED_HTTP_STATUS") {
				t.Fatalf("final failure diagnostics lost: %+v", items[0])
			}
		})
	}
}

func TestHostRetriesGetFreshTimeouts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ProxyUserID, cfg.TimeoutSeconds, cfg.MaxRetries, cfg.RetryIntervalSeconds = 42, 1, 1, 3
	calls, waits := 0, 0
	var firstDeadline time.Time
	probe := testProbe(func(ctx context.Context, _ Target, _ Config) Outcome {
		calls++
		if calls == 1 {
			firstDeadline, _ = ctx.Deadline()
			<-ctx.Done()
			return Outcome{Status: "interrupted", ErrorCode: "CONNECTIVITY_INTERRUPTED"}
		}
		deadline, ok := ctx.Deadline()
		if !ok || !deadline.After(firstDeadline) {
			t.Error("retry reused the expired first-attempt deadline")
		}
		return Outcome{Status: "connected"}
	})
	service, repository, item := retryFixture(t, cfg, probe)
	service.retryWait = func(ctx context.Context, delay time.Duration) error {
		waits++
		if delay != 3*time.Second || ctx.Err() != nil {
			t.Errorf("delay did not use live batch context: %s %v", delay, ctx.Err())
		}
		return ctx.Err()
	}
	service.process(item)
	if calls != 2 || waits != 1 || repository.attempts()[0].Status != "connected" {
		t.Fatalf("timeout was not retried: calls=%d waits=%d attempts=%+v", calls, waits, repository.attempts())
	}
}

func TestHostRetriesDoNotBypassStoppedQueue(t *testing.T) {
	for _, phase := range []string{"initial", "retry"} {
		t.Run(phase, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.ProxyUserID = 42
			calls, waits, wantCalls := 0, 0, 0
			service, repository, item := retryFixture(t, cfg, testProbe(func(context.Context, Target, Config) Outcome {
				calls++
				return Outcome{Status: "failed", ErrorCode: "CONNECTIVITY_PROBE_FAILED"}
			}))
			if phase == "initial" {
				if err := service.queue.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			} else {
				wantCalls = 1
			}
			service.retryWait = func(ctx context.Context, _ time.Duration) error {
				waits++
				return service.queue.Shutdown(ctx)
			}
			service.process(item)
			if calls != wantCalls || waits != wantCalls || repository.attempts()[0].Status != "error" {
				t.Fatalf("stopped queue bypassed: calls=%d waits=%d attempts=%+v", calls, waits, repository.attempts())
			}
		})
	}
}
