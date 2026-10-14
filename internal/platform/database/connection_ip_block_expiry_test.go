package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConnectionIPBlockExpiryResolvesQueuedManualUnblock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	owner := createTestUser(t, store, 60_005)
	now := time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC)
	digest := strings.Repeat("c", 64)
	block, _, _, err := store.BeginConnectionIPBlock(ctx, connections.CreateIPBlockInput{UserID: owner.ID,
		NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee", IPDigest: digest, SealedIP: "cipher", ExpiresAt: now},
		providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: connections.BlockOperationKind,
			IdempotencyKey: "create-expiry", RequestFingerprint: "0123456789abcdef",
			Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: digest}}}, now.Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	unblock, _, err := store.BeginConnectionIPUnblock(ctx, block.ID, owner.ID, providerops.CreateInput{
		ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: connections.UnblockOperationKind,
		IdempotencyKey: "unblock-expiry", RequestFingerprint: "abcdef0123456789",
		Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: digest}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinalizeConnectionIPBlockExpiry(ctx, block.ID, true, now); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ProviderOperationByID(ctx, unblock.Receipt.ID)
	if err != nil || completed.Receipt.Status != string(providerops.StatusSucceeded) {
		t.Fatalf("unblock completion = (%+v, %v)", completed, err)
	}
	items, err := store.ProviderOperationItems(ctx, unblock.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].Status != providerops.StatusSucceeded {
		t.Fatalf("unblock items = (%+v, %v)", items, err)
	}
	if _, err := store.ConnectionIPBlockByID(ctx, block.ID); err == nil {
		t.Fatal("expired block row was retained")
	}
}
