package upstreamqueue

import (
	"context"
	"testing"
	"time"
)

func TestOccupiedKeyDoesNotBlockIndependentCall(t *testing.T) {
	q := startTestQueue(t, Config{Name: "keyed", Capacity: 8, Workers: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan string, 3)
	release := make(chan struct{})
	submit := func(key, label string, hold bool) {
		go func() {
			_ = Execute(WithKey(ctx, key), q, func(ctx context.Context) error {
				started <- label
				if hold {
					select {
					case <-release:
					case <-ctx.Done():
					}
				}
				return nil
			})
		}()
	}
	submit("account-a", "a1", true)
	select {
	case got := <-started:
		if got != "a1" {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	submit("account-a", "a2", false)
	submit("account-b", "b1", false)
	select {
	case got := <-started:
		if got != "b1" {
			t.Fatalf("same key overtook its predecessor: %s", got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	close(release)
	select {
	case got := <-started:
		if got != "a2" {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
