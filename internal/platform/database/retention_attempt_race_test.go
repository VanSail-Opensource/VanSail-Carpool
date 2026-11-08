package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMaintenanceRechecksAttemptCommittedWhileCandidateLockWaits(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s := newTestStore(t)
	u := createTestUser(t, s, 497683)
	now := time.Now().UTC()
	old := now.Add(-40 * 24 * time.Hour)
	op, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: u.ID, OwnerUserID: u.ID,
		Kind: "retention-race", IdempotencyKey: "race", RequestFingerprint: "retention-race-proof",
		SealedTarget: "synthetic-encrypted-target", Items: []providerops.ItemInput{{Key: "one", TargetType: "user", TargetID: u.ID}},
	}, old)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the same row lock as BeginAttempt after its durable mutation but
	// before COMMIT. Maintenance must not retain its earlier queued snapshot.
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status='processing',attempts=1,attempt_started_at=$2 WHERE id=$1`, op.Receipt.ID, stamp(old)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'SELECT p.id FROM provider_operations p')>0 AND strpos(query,'FOR UPDATE OF p')>0)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("maintenance did not fence its candidate against BeginAttempt")
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	got, err := s.ProviderOperationByID(ctx, op.Receipt.ID)
	if err != nil || got.Receipt.Status != "processing" || got.Attempts != 1 {
		t.Fatal("committed attempt was inferred failed from age", err)
	}
	var retained bool
	if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind='provider_operation' AND payload::jsonb->>'operationId'=$1 AND payload::jsonb->>'sealedTarget'='synthetic-encrypted-target')`, op.Receipt.ID).Scan(&retained); err != nil || !retained {
		t.Fatal("committed attempt lost its encrypted continuation", retained, err)
	}
}
