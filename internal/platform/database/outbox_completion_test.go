package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestPostgresConcurrentBusyCompletionAndHeartbeats(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	jobs := make([]model.OutboxJob, 0, 12)
	for i := range 12 {
		if err := s.EnqueueOutbox(ctx, "completion_test", fmt.Sprintf(`{"index":%d}`, i), now); err != nil {
			t.Fatal(err)
		}
	}
	for range 12 {
		job, err := s.ClaimOutboxJob(ctx, now.Add(time.Second))
		if err != nil || job == nil {
			t.Fatalf("claim: %+v %v", job, err)
		}
		jobs = append(jobs, *job)
	}
	before := postgres.Snapshot(s.db.Driver())
	start := make(chan struct{})
	results := make(chan error, len(jobs)*3)
	for _, job := range jobs {
		for range 2 {
			go func() {
				<-start
				results <- s.CompleteOutboxClaim(ctx, job, outbox.ErrBusy, now)
			}()
		}
		go func() {
			<-start
			_, err := s.RenewOutboxClaim(ctx, job, now)
			results <- err
		}()
	}
	close(start)
	for range len(jobs) * 3 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	after := postgres.Snapshot(s.db.Driver())
	if after.Retries != before.Retries {
		t.Fatalf("row-locked completion required serialization retries: %d", after.Retries-before.Retries)
	}
	for _, old := range jobs {
		var status string
		var attempts int
		if err := s.db.QueryRowContext(ctx, `SELECT status,attempts FROM outbox_jobs WHERE id=?`, old.ID).Scan(&status, &attempts); err != nil || status != "pending" || attempts != 0 {
			t.Fatalf("busy job = %s %d %v", status, attempts, err)
		}
	}
	current, err := s.ClaimOutboxJob(ctx, now.Add(2*time.Second))
	if err != nil || current == nil {
		t.Fatalf("reclaim: %+v %v", current, err)
	}
	for _, old := range jobs {
		if err := s.CompleteOutboxClaim(ctx, old, nil, now); err != nil {
			t.Fatal(err)
		}
	}
	if owned, err := s.RenewOutboxClaim(ctx, *current, now); err != nil || !owned {
		t.Fatalf("stale completion overwrote new generation: %v %v", owned, err)
	}
}
