package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestDeleteOutboxPreservesUnfinishedCommandAndItsEncryptedContinuation(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	owner := createTestAdmin(t, store, 497710)
	input := providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: "test_recovery",
		IdempotencyKey: "delete-recovery", RequestFingerprint: "delete-recovery-proof", SealedTarget: "encrypted-continuation",
		Items: []providerops.ItemInput{{Key: "remote", TargetType: "user", TargetID: owner.ID}}}
	op, _, err := store.CreateProviderOperation(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	var jobID string
	if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE payload::jsonb->>'operationId'=$1`, op.Receipt.ID).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOutboxJob(ctx, jobID, owner.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("queued command lost its continuation", err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, op.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, op.Receipt.ID, "remote", now); err != nil {
		t.Fatal(err)
	}
	// A failed parent is insufficient proof while its external child is uncertain.
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='failed' WHERE id=$1`, op.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed' WHERE id=$1`, jobID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOutboxJob(ctx, jobID, owner.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("failed job erased an attempted child", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='processing' WHERE id=$1`, op.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperationItem(ctx, op.Receipt.ID, "remote", providerops.Completion{Status: providerops.StatusSucceeded}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, op.Receipt.ID, providerops.Completion{Status: providerops.StatusSucceeded}, now); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOutboxJob(ctx, jobID, owner.ID); err != nil {
		t.Fatal("settled continuation could not be removed", err)
	}
	receipt, replayed, err := store.CreateOrReplayProviderOperation(ctx, input, now)
	if err != nil || !replayed || receipt.Receipt.ID != op.Receipt.ID || receipt.Receipt.Status != "succeeded" {
		t.Fatal("removing the job destroyed the exact replay receipt", receipt, replayed, err)
	}
}

func TestDeleteOutboxKeepsFinancialAndCleanupLanesButAllowsSettledArtifacts(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	owner := createTestAdmin(t, store, 497713)
	for _, kind := range []string{"xnode_hourly_settle", "draw_raffle_settle", "account_purge", "account_cleanup_probe"} {
		if err := store.EnqueueOutbox(ctx, kind, `{"userId":"retained-owner"}`, now); err != nil {
			t.Fatal(err)
		}
		var id string
		if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind=$1`, kind).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteOutboxJob(ctx, id, owner.ID); !errors.Is(err, ErrConflict) {
			t.Fatal("authoritative lane was treated as optional delivery", kind, err)
		}
		if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='done' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteOutboxJob(ctx, id, owner.ID); err != nil {
			t.Fatal("legacy completed artifact cannot be removed", kind, err)
		}
	}
}

func TestLegacyEmbyRetrySurvivesManualAndAutomaticJobCleanup(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now, old := time.Now().UTC(), time.Now().UTC().Add(-10*24*time.Hour)
	owner := createTestAdmin(t, store, 497711)
	if _, err := store.AdjustBalance(ctx, owner.ID, 1000, "legacy-recovery-seed", "seed", old); err != nil {
		t.Fatal(err)
	}
	account, _, err := store.QueueEmbySetup(ctx, emby.QueueSetupInput{ID: "legacy-recovery", UserID: owner.ID,
		BaseUsername: "497711", SetupPriceTXBMinor: 275, PasswordCiphertext: "encrypted-password", PasswordContext: "account-context"}, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE emby_accounts SET create_attempted=1,status='pending_review' WHERE id=$1`, account.ID); err != nil {
		t.Fatal(err)
	}
	var jobID string
	if err := store.DB().QueryRowContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10,updated_at=$1 WHERE kind='emby_provision_account' RETURNING id`, stamp(old)).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.DeleteOutboxJob(ctx, jobID, owner.ID); !errors.Is(err, ErrConflict) {
			t.Fatal("legacy charged setup lost its explicit retry", err)
		}
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		var retained int
		if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE id=$1`, jobID).Scan(&retained); err != nil || retained != 1 {
			t.Fatal("maintenance erased uncertain setup job", retained, err)
		}
		balance, err := store.Balance(ctx, owner.ID)
		if err != nil || balance.Minor != "725" {
			t.Fatal("cleanup changed an uncertain setup charge", balance, err)
		}
	}
}

func TestOutboxDeletionAndActorAuditCommitTogether(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestAdmin(t, store, 497714)
	if err := store.EnqueueOutbox(ctx, "xnode_sync_user", `{"userId":"retained-owner"}`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind='xnode_sync_user'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOutboxJob(ctx, id, "missing-audit-actor"); err == nil {
		t.Fatal("deletion committed without its required actor audit")
	}
	var retained int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE id=$1`, id).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("audit failure did not roll back deletion", retained, err)
	}
	if err := store.DeleteOutboxJob(ctx, id, owner.ID); err != nil {
		t.Fatal(err)
	}
	var audit int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM audit_events WHERE action='outbox.delete' AND actor_user_id=$1 AND target_id=$2`, owner.ID, id).Scan(&audit); err != nil || audit != 1 {
		t.Fatal("job deletion lost its actor attribution", audit, err)
	}
}
