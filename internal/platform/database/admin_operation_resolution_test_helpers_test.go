package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func resolutionFixture(t *testing.T, kind string, outcomes ...providerops.Status) (*Store, AdminOperationResolutionInput, time.Time) {
	t.Helper()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89104)
	owner := createTestUser(t, store, 89105)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	items := make([]providerops.ItemInput, len(outcomes))
	for index := range outcomes {
		items[index] = providerops.ItemInput{Key: fmt.Sprintf("target-%d", index), TargetType: "user", TargetID: owner.ID}
	}
	operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID, OwnerUserID: owner.ID,
		Kind: kind, IdempotencyKey: "original-command", RequestFingerprint: "original-command-fingerprint", Items: items}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	for index, outcome := range outcomes {
		if outcome == providerops.StatusQueued {
			continue
		}
		if _, err = store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, items[index].Key, now); err != nil {
			t.Fatal(err)
		}
		if outcome != providerops.StatusProcessing {
			if _, err = store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, items[index].Key,
				providerops.Completion{Status: outcome, ResultJSON: `{"evidence":9007199254740993}`}, now); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = store.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{Status: providerops.StatusPendingReview,
		ErrorCode: "REVIEW_REQUIRED", ResultJSON: `{"retained":9007199254740993}`}, now); err != nil {
		t.Fatal(err)
	}
	return store, AdminOperationResolutionInput{ActorUserID: actor.ID, OperationID: operation.Receipt.ID,
		IdempotencyKey: "repair-key", RequestFingerprint: "repair-request-fingerprint", Resolution: "succeeded", Reason: "repair receipt from retained evidence"}, now.Add(time.Minute)
}

func assertNoResolutionWrites(t *testing.T, store *Store, operationID string) {
	t.Helper()
	operation, err := store.ProviderOperationByID(t.Context(), operationID)
	if err != nil || operation.Receipt.Status != string(providerops.StatusPendingReview) || operation.Receipt.ErrorCode == nil {
		t.Fatal("rejected resolution changed retained receipt", operation, err)
	}
	var replays, audits int
	if err := store.DB().QueryRowContext(t.Context(), `SELECT
		(SELECT COUNT(*) FROM provider_operation_replays WHERE operation_id=$1),
		(SELECT COUNT(*) FROM audit_events WHERE action='provider_operation.resolve' AND target_id=$1)`, operationID).Scan(&replays, &audits); err != nil || replays != 0 || audits != 0 {
		t.Fatal("rejected resolution left a replay or success audit", replays, audits, err)
	}
}
