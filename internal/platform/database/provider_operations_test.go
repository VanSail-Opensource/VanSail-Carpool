package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestProviderOperationReplayAndAttemptLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	owner := createTestUser(t, store, 50_001)
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	input := providerops.CreateInput{
		ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: "test_operation",
		IdempotencyKey: "operation-key", RequestFingerprint: "0123456789abcdef",
		SealedTarget: "sealed-ciphertext",
		Items:        []providerops.ItemInput{{Key: "target-1", TargetType: "purchase", TargetID: "purchase-1"}},
	}
	operation, replayed, err := store.CreateProviderOperation(ctx, input, now)
	if err != nil || replayed || operation.Receipt.Status != string(providerops.StatusQueued) {
		t.Fatalf("CreateProviderOperation() = (%+v, %t, %v)", operation, replayed, err)
	}

	replayCases := []struct {
		name        string
		fingerprint string
		wantReplay  bool
		wantErr     error
	}{
		{name: "matching fingerprint", fingerprint: input.RequestFingerprint, wantReplay: true},
		{name: "conflicting fingerprint", fingerprint: "fedcba9876543210", wantErr: ErrConflict},
	}
	for _, test := range replayCases {
		t.Run(test.name, func(t *testing.T) {
			replayInput := input
			replayInput.RequestFingerprint = test.fingerprint
			got, gotReplay, gotErr := store.CreateOrReplayProviderOperation(ctx, replayInput, now.Add(time.Minute))
			if !errors.Is(gotErr, test.wantErr) || gotReplay != test.wantReplay {
				t.Fatalf("CreateOrReplayProviderOperation() = (%+v, %t, %v), want replay=%t err=%v", got, gotReplay, gotErr, test.wantReplay, test.wantErr)
			}
			if gotErr == nil && got.Receipt.ID != operation.Receipt.ID {
				t.Fatalf("replayed operation ID = %q, want %q", got.Receipt.ID, operation.Receipt.ID)
			}
		})
	}

	started, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now.Add(2*time.Minute))
	if err != nil || started.Receipt.Status != string(providerops.StatusProcessing) || started.Attempts != 1 {
		t.Fatalf("BeginProviderOperationAttempt() = (%+v, %v)", started, err)
	}
	item, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "target-1", now.Add(3*time.Minute))
	if err != nil || item.Status != providerops.StatusProcessing || item.AttemptStartedAt == nil {
		t.Fatalf("BeginProviderOperationItemAttempt() = (%+v, %v)", item, err)
	}
	item, err = store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, item.Key,
		providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: `{"applied":true}`}, now.Add(4*time.Minute))
	if err != nil || item.Status != providerops.StatusSucceeded || item.CompletedAt == nil {
		t.Fatalf("CompleteProviderOperationItem() = (%+v, %v)", item, err)
	}
	completed, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID,
		providerops.Completion{Status: providerops.StatusSucceeded, ProviderReference: "remote-1"}, now.Add(5*time.Minute))
	if err != nil || completed.Receipt.Status != string(providerops.StatusSucceeded) || completed.Receipt.CompletedAt == nil {
		t.Fatalf("CompleteProviderOperation() = (%+v, %v)", completed, err)
	}
	receipt, err := store.ProviderOperationForOwner(ctx, operation.Receipt.ID, owner.ID)
	if err != nil || receipt.ID != operation.Receipt.ID || receipt.Status != string(providerops.StatusSucceeded) {
		t.Fatalf("ProviderOperationForOwner() = (%+v, %v)", receipt, err)
	}
	var jobs, replays int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=?
		AND json_extract(payload,'$.operationId')=? AND json_extract(payload,'$.sealedTarget')=?`,
		providerops.OutboxKind, operation.Receipt.ID, input.SealedTarget).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operation_replays WHERE operation_id=?`, operation.Receipt.ID).Scan(&replays); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 || replays != 1 {
		t.Fatalf("durable evidence = jobs:%d replays:%d, want 1/1", jobs, replays)
	}
}

func TestHostRemarkOperationsBlockFreshCommandsWhileOutcomeIsUnresolved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	actor := createTestUser(t, store, 50_002)
	now := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	input := providerops.CreateInput{ActorUserID: actor.ID, Kind: providerops.KindHostRemarkUpdate,
		IdempotencyKey: "host-first", RequestFingerprint: "1111111111111111",
		Items: []providerops.ItemInput{{Key: "host", TargetType: "remnawave_host", TargetID: "host-1"}}}
	first, _, err := store.CreateProviderOperation(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, first.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, first.Receipt.ID, "host", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperationItem(ctx, first.Receipt.ID, "host",
		providerops.Completion{Status: providerops.StatusPendingReview}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, first.Receipt.ID,
		providerops.Completion{Status: providerops.StatusPendingReview}, now); err != nil {
		t.Fatal(err)
	}
	second := input
	second.IdempotencyKey = "host-second"
	second.RequestFingerprint = "2222222222222222"
	if _, _, err := store.CreateProviderOperation(ctx, second, now.Add(time.Minute)); err == nil {
		t.Fatal("fresh host command was accepted while the first outcome required review")
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='failed' WHERE id=?`, first.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateProviderOperation(ctx, second, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("resolved host command remained blocked: %v", err)
	}
}
