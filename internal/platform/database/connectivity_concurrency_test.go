package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func TestConnectivityConcurrentFinishKeepsExactlyOneTerminalResult(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	attempt := connectivityTestAttempt(1, connectivityTestHost, now)
	seedConnectivityAttempt(t, store, attempt)
	outcomes := []connectivity.Outcome{connectivityOutcome(), {Status: "interrupted", ErrorCode: "CONNECTIVITY_INTERRUPTED"}}
	start, results := make(chan struct{}), make(chan error, len(outcomes))
	for _, outcome := range outcomes {
		go func() {
			<-start
			results <- store.FinishConnectivityAttempt(ctx, attempt.ID, outcome, now.Add(time.Second))
		}()
	}
	close(start)
	successful := 0
	for range outcomes {
		err := <-results
		if err == nil {
			successful++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatalf("concurrent finish=%v", err)
		}
	}
	if successful != 1 {
		t.Fatalf("successful transitions=%d", successful)
	}
	latest, err := store.LatestConnectivityAttempts(ctx, attempt.ConfigHash, now.Add(time.Minute))
	if err != nil || len(latest) != 1 || latest[0].FinishedAt == nil || (latest[0].Status != "connected" && latest[0].Status != "interrupted") {
		t.Fatalf("sealed concurrent result=%+v,%v", latest, err)
	}
}
