package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestListAdminOperationsForUserIncludesOpenBulkItemsOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	actor := createTestUser(t, store, 43001)
	target := createTestUser(t, store, 43002)
	other := createTestUser(t, store, 43003)

	owned, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID,
		OwnerUserID: target.ID, Kind: providerops.KindAdminEntitlementEdit, IdempotencyKey: "owned-open",
		RequestFingerprint: "owned-fingerprint-01", Items: []providerops.ItemInput{{Key: target.ID, TargetType: "user", TargetID: target.ID}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	bulk, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID,
		Kind: providerops.KindAdminBulkExtension, IdempotencyKey: "bulk-open", RequestFingerprint: "bulk-fingerprint-01",
		Items: []providerops.ItemInput{{Key: target.ID, TargetType: "user", TargetID: target.ID}}}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	closed, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID,
		OwnerUserID: target.ID, Kind: providerops.KindAdminEntitlementRefund, IdempotencyKey: "closed",
		RequestFingerprint: "closed-fingerprint-01", Items: []providerops.ItemInput{{Key: target.ID, TargetType: "user", TargetID: target.ID}}}, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, closed.Receipt.ID, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, closed.Receipt.ID, providerops.Completion{Status: providerops.StatusSucceeded}, now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID,
		OwnerUserID: other.ID, Kind: providerops.KindAdminEntitlementEdit, IdempotencyKey: "other-open",
		RequestFingerprint: "other-fingerprint-01", Items: []providerops.ItemInput{{Key: other.ID, TargetType: "user", TargetID: other.ID}}}, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}

	operations, err := store.ListAdminOperationsForUser(ctx, target.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 2 {
		t.Fatalf("operations = %+v, want owned and bulk open receipts", operations)
	}
	seen := map[string]bool{}
	for _, operation := range operations {
		seen[operation.ID] = true
		if operation.Status == string(providerops.StatusSucceeded) {
			t.Fatalf("closed operation leaked into projection: %+v", operation)
		}
	}
	if !seen[owned.Receipt.ID] || !seen[bulk.Receipt.ID] {
		t.Fatalf("operations = %+v", operations)
	}
}
