package upstreamqueue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{name: "valid", config: Config{Name: "provider", Capacity: 3, MinInterval: time.Millisecond}},
		{name: "missing name", config: Config{Capacity: 1}, wantErr: true},
		{name: "zero capacity", config: Config{Name: "provider"}, wantErr: true},
		{name: "negative interval", config: Config{Name: "provider", Capacity: 1, MinInterval: -time.Second}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue, err := New(test.config)
			if (err != nil) != test.wantErr {
				t.Fatalf("New() error = %v, wantErr %t", err, test.wantErr)
			}
			if !test.wantErr && (queue.Name() != test.config.Name || queue.Capacity() != test.config.Capacity || queue.MinInterval() != test.config.MinInterval) {
				t.Fatalf("New() config = (%q, %d, %s)", queue.Name(), queue.Capacity(), queue.MinInterval())
			}
		})
	}
}

func TestDoOutcomesAndPanicRecovery(t *testing.T) {
	queue := startTestQueue(t, Config{Name: "test", Capacity: 4})
	sentinel := errors.New("provider failed")
	tests := []struct {
		name      string
		call      func(context.Context) (string, error)
		want      string
		wantError error
	}{
		{name: "result", call: func(context.Context) (string, error) { return "ok", nil }, want: "ok"},
		{name: "provider error", call: func(context.Context) (string, error) { return "", sentinel }, wantError: sentinel},
		{name: "panic", call: func(context.Context) (string, error) { panic("secret payload") }, wantError: ErrTaskPanic},
		{name: "worker continues", call: func(context.Context) (string, error) { return "after panic", nil }, want: "after panic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Do(context.Background(), queue, test.call)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Do() error = %v, want %v", err, test.wantError)
			}
			if got != test.want {
				t.Fatalf("Do() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestQueueBackpressureHonorsCallerCancellation(t *testing.T) {
	queue := startTestQueue(t, Config{Name: "bounded", Capacity: 1})
	setupCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- Execute(setupCtx, queue, func(ctx context.Context) error {
			close(firstStarted)
			select {
			case <-releaseFirst:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	<-firstStarted

	var secondCalls atomic.Int32
	secondDone := make(chan error, 1)
	// Wait for actual admission, independent of the scheduler's storage layout.
	if _, err := queue.enqueue(setupCtx, call{ctx: setupCtx,
		run: func(context.Context) {
			secondCalls.Add(1)
			secondDone <- nil
		}, reject: func(err error) { secondDone <- err },
	}); err != nil {
		t.Fatal("second admission", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	var backpressuredCalls atomic.Int32
	_, err := queue.enqueue(ctx, call{ctx: ctx,
		run:    func(context.Context) { backpressuredCalls.Add(1) },
		reject: func(error) {},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Execute() error = %v, want deadline exceeded", err)
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second call: %v", err)
	}
	if secondCalls.Load() != 1 {
		t.Fatalf("second call count = %d, want 1", secondCalls.Load())
	}
	if backpressuredCalls.Load() != 0 {
		t.Fatalf("backpressured call count = %d, want 0", backpressuredCalls.Load())
	}
}

func TestCancellationReachesActiveCall(t *testing.T) {
	tests := []struct {
		name            string
		cancelLifecycle bool
	}{
		{name: "caller cancellation"},
		{name: "lifecycle cancellation", cancelLifecycle: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue, err := New(Config{Name: "cancel", Capacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			lifecycle, stopLifecycle := context.WithCancel(context.Background())
			caller, stopCaller := context.WithCancel(context.Background())
			defer stopLifecycle()
			defer stopCaller()
			if err := queue.Start(lifecycle); err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- Execute(caller, queue, func(ctx context.Context) error {
					close(started)
					<-ctx.Done()
					return ctx.Err()
				})
			}()
			<-started
			if test.cancelLifecycle {
				stopLifecycle()
			} else {
				stopCaller()
			}
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("active call error = %v, want canceled", err)
			}
			if err := queue.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})
	}
}

func TestQueuePacesCallStarts(t *testing.T) {
	const interval = 30 * time.Millisecond
	queue := startTestQueue(t, Config{Name: "paced", Capacity: 2, MinInterval: interval})
	starts := make([]time.Time, 0, 2)
	for range 2 {
		if err := Execute(context.Background(), queue, func(context.Context) error {
			starts = append(starts, time.Now())
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := starts[1].Sub(starts[0]); elapsed < interval-5*time.Millisecond {
		t.Fatalf("call starts separated by %s, want at least %s", elapsed, interval-5*time.Millisecond)
	}
}

func TestDoRequiresRunningQueue(t *testing.T) {
	queue, err := New(Config{Name: "stopped", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Do(context.Background(), queue, func(context.Context) (int, error) { return 1, nil })
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Do() error = %v, want ErrNotRunning", err)
	}
}

func TestDoValidatesArguments(t *testing.T) {
	queue, err := New(Config{Name: "validation", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "nil context", call: func() error {
			_, err := Do[int](nil, queue, func(context.Context) (int, error) { return 1, nil })
			return err
		}},
		{name: "nil queue", call: func() error {
			_, err := Do(context.Background(), (*Queue)(nil), func(context.Context) (int, error) { return 1, nil })
			return err
		}},
		{name: "nil function", call: func() error {
			_, err := Do[int](context.Background(), queue, nil)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("Do() error = nil")
			}
		})
	}
}

func TestQueueStartAndShutdownState(t *testing.T) {
	queue, err := New(Config{Name: "state", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("second Start() error = %v, want ErrAlreadyStarted", err)
	}
	if err := queue.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := queue.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}

	neverStarted, err := New(Config{Name: "never-started", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := neverStarted.Shutdown(context.Background()); err != nil {
		t.Fatalf("pre-start Shutdown() error = %v", err)
	}
	if err := neverStarted.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("Start() after Shutdown error = %v, want ErrAlreadyStarted", err)
	}
}

func startTestQueue(t *testing.T, config Config) *Queue {
	t.Helper()
	queue, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := queue.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	return queue
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not met before timeout")
		}
		time.Sleep(time.Millisecond)
	}
}
