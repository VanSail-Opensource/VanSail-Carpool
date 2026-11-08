package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func cleanupOperationInput(owner string) providerops.CreateInput {
	return providerops.CreateInput{ActorUserID: owner, OwnerUserID: owner, Kind: "test_operation", IdempotencyKey: "cleanup-fence", RequestFingerprint: "0123456789abcdef", Items: []providerops.ItemInput{{Key: "one", TargetType: "user", TargetID: owner}}}
}

func TestCleanupProviderIntentWaitsForLifecycleFenceThenRejects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s := newTestStore(t)
	u := createTestUser(t, s, 497671)
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, u.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := s.CreateProviderOperation(ctx, cleanupOperationInput(u.ID), time.Now().UTC())
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'ORDER BY id FOR SHARE')>0)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("new intent did not acquire lifecycle share lock")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1,last_purged_at=$2 WHERE id=$1`, u.ID, stamp(time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("intent crossed committed purge fence", err)
	}
	var count int
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM provider_operations WHERE owner_user_id=$1`, u.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestCleanupProviderExactReceiptReplaysAfterOwnerFence(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497672)
	input := cleanupOperationInput(u.ID)
	op, _, err := s.CreateProviderOperation(ctx, input, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1 WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	got, replay, err := s.CreateProviderOperation(ctx, input, time.Now().UTC())
	if err != nil || !replay || got.Receipt.ID != op.Receipt.ID {
		t.Fatal(got, replay, err)
	}
	input.IdempotencyKey = "new-forbidden"
	if _, _, err = s.CreateProviderOperation(ctx, input, time.Now().UTC()); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal(err)
	}
	var count int
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("replay queued mutation", count, err)
	}
}

func TestCleanupAdmissionFencesClaimedProviderBeforeBeginAttempt(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497673)
	at := time.Now().UTC().Truncate(time.Microsecond)
	op, _, err := s.CreateProviderOperation(ctx, cleanupOperationInput(u.ID), at)
	if err != nil {
		t.Fatal(err)
	}
	// Model the real dispatcher pause after a durable claim and its earlier
	// account-active check, but before the handler's BeginAttempt CAS.
	job, err := s.ClaimOutboxJob(ctx, at)
	if err != nil || job == nil || job.Kind != "provider_operation" {
		t.Fatal(job, err)
	}
	permitted, err := s.AccountOperationPermitted(ctx, op.Receipt.ID)
	if err != nil || !permitted {
		t.Fatal(permitted, err)
	}
	admitTestAccountCleanup(t, s, u, at)
	if _, err = s.BeginProviderOperationAttempt(ctx, op.Receipt.ID, at); !errors.Is(err, ErrConflict) {
		t.Fatal("claimed stale dispatcher crossed admission", err)
	}
	var status string
	var attempts int
	if err = s.DB().QueryRowContext(ctx, `SELECT status,attempts FROM provider_operations WHERE id=$1`, op.Receipt.ID).Scan(&status, &attempts); err != nil || status != "failed" || attempts != 0 {
		t.Fatal("unattempted parent was not superseded atomically", status, attempts, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT status FROM outbox_jobs WHERE id=$1`, job.ID).Scan(&status); err != nil || status != "processing" {
		t.Fatal("processing claim was silently cleared", status, err)
	}
}
