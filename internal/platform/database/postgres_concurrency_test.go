package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPostgresConcurrentDebits(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user := createTestUser(t, s, 917001)
	if _, err := s.AdjustBalance(ctx, user.ID, 100, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{"first", "second"} {
		go func(id string) {
			<-start
			_, err := s.DeductBalance(ctx, user.ID, 80, id, "debit", now)
			results <- err
		}(id)
	}
	close(start)
	success, insufficient := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrInsufficientBalance) {
			insufficient++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || insufficient != 1 {
		t.Fatalf("success=%d insufficient=%d", success, insufficient)
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "20" {
		t.Fatalf("balance=%+v error=%v", balance, err)
	}
	entries, err := s.ListLedger(ctx, user.ID, 10)
	if err != nil || len(entries) != 2 {
		t.Fatalf("ledger=%d error=%v", len(entries), err)
	}
}

func TestPostgresOutboxGenerationSurvivesAttemptReset(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.EnqueueOutbox(ctx, "fence_test", `{"userId":"synthetic"}`, now); err != nil {
		t.Fatal(err)
	}
	old, err := s.ClaimOutboxJob(ctx, now.Add(time.Second))
	if err != nil || old == nil {
		t.Fatal("first claim", err)
	}
	if err = s.RecoverOutbox(ctx, now.Add(2*time.Second), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE outbox_jobs SET attempts=0 WHERE id=?`, old.ID); err != nil {
		t.Fatal(err)
	}
	current, err := s.ClaimOutboxJob(ctx, now.Add(3*time.Second))
	if err != nil || current == nil {
		t.Fatal("recovered claim", err)
	}
	if current.Attempts != old.Attempts || current.ClaimGeneration <= old.ClaimGeneration {
		t.Fatal("claim generation did not fence reset attempts")
	}
	if err = s.CompleteOutboxClaim(ctx, *old, nil, now); err != nil {
		t.Fatal(err)
	}
	if owned, err := s.RenewOutboxClaim(ctx, *old, now); err != nil || owned {
		t.Fatal("stale heartbeat accepted", err)
	}
	if owned, err := s.RenewOutboxClaim(ctx, *current, now); err != nil || !owned {
		t.Fatal("current claim lost", err)
	}
	if err = s.CompleteOutboxClaim(ctx, *current, nil, now); err != nil {
		t.Fatal(err)
	}
}
