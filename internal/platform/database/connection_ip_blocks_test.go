package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConnectionIPBlockAtomicCreationReplayAndIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	owner := createTestUser(t, store, 60_001)
	other := createTestUser(t, store, 60_002)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	digest := strings.Repeat("a", 64)
	input := connections.CreateIPBlockInput{UserID: owner.ID, NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee",
		IPDigest: digest, SealedIP: "vault:v1:ciphertext", ExpiresAt: now.Add(connections.BlockDuration)}
	command := providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: connections.BlockOperationKind,
		IdempotencyKey: "block-key", RequestFingerprint: "0123456789abcdef",
		Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: digest}}}

	block, operation, replayed, err := store.BeginConnectionIPBlock(ctx, input, command, now)
	if err != nil || replayed || block.ExpiresAt.Sub(block.CreatedAt) != connections.BlockDuration {
		t.Fatalf("BeginConnectionIPBlock() = (%+v, %+v, %t, %v)", block, operation, replayed, err)
	}
	replayedBlock, replayedOperation, replayed, err := store.BeginConnectionIPBlock(ctx, input, command, now.Add(time.Second))
	if err != nil || !replayed || replayedBlock.ID != block.ID || replayedOperation.Receipt.ID != operation.Receipt.ID {
		t.Fatalf("replay = (%+v, %+v, %t, %v)", replayedBlock, replayedOperation, replayed, err)
	}
	duplicate := command
	duplicate.IdempotencyKey = "different-key"
	duplicate.RequestFingerprint = "fedcba9876543210"
	if _, _, _, err := store.BeginConnectionIPBlock(ctx, input, duplicate, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate target error = %v", err)
	}
	if _, err := store.ConnectionIPBlockForUser(ctx, block.ID, other.ID); !errors.Is(err, connections.ErrIPBlockNotFound) {
		t.Fatalf("cross-owner lookup error = %v", err)
	}

	var rows, immediate, expiry, plaintext int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM connection_ip_blocks`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND json_extract(payload,'$.operationId')=?`,
		providerops.OutboxKind, operation.Receipt.ID).Scan(&immediate); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND json_extract(payload,'$.blockId')=?`,
		connections.BlockExpiryOutboxKind, block.ID).Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM connection_ip_blocks WHERE sealed_ip LIKE '%203.0.113.8%'
		OR EXISTS(SELECT 1 FROM outbox_jobs WHERE payload LIKE '%203.0.113.8%')
		OR EXISTS(SELECT 1 FROM provider_operation_items WHERE target_id LIKE '%203.0.113.8%')`).Scan(&plaintext); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || immediate != 1 || expiry != 1 || plaintext != 0 {
		t.Fatalf("durability rows=%d immediate=%d expiry=%d plaintext=%d", rows, immediate, expiry, plaintext)
	}
}

func TestConnectionIPUnblockCreationAndManualCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	owner := createTestUser(t, store, 60_003)
	admin := createTestUser(t, store, 60_004)
	now := time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC)
	digest := strings.Repeat("b", 64)
	block, _, _, err := store.BeginConnectionIPBlock(ctx, connections.CreateIPBlockInput{UserID: owner.ID,
		NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee", IPDigest: digest, SealedIP: "cipher", ExpiresAt: now.Add(72 * time.Hour)},
		providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: connections.BlockOperationKind,
			IdempotencyKey: "create", RequestFingerprint: "0123456789abcdef",
			Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: digest}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	command := providerops.CreateInput{ActorUserID: admin.ID, OwnerUserID: owner.ID, Kind: connections.UnblockOperationKind,
		IdempotencyKey: "unblock", RequestFingerprint: "abcdef0123456789",
		Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: digest}}}
	operation, replayed, err := store.BeginConnectionIPUnblock(ctx, block.ID, owner.ID, command, now)
	if err != nil || replayed || operation.ActorUserID != admin.ID || operation.OwnerUserID != owner.ID {
		t.Fatalf("unblock = (%+v, %t, %v)", operation, replayed, err)
	}
	if _, _, err := store.BeginConnectionIPUnblock(ctx, block.ID, owner.ID, command, now); err != nil {
		t.Fatalf("idempotent unblock replay: %v", err)
	}
	operation, err = store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "ip", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteConnectionIPBlockOperation(ctx, block.ID, operation.Receipt.ID, "ip",
		connections.BlockOperationCompletion{Operation: providerops.Completion{Status: providerops.StatusSucceeded},
			ItemStatus: providerops.StatusSucceeded, RemoveBlock: true}, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	var expiry, active int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE id=?`, block.ExpiryJobID).Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM connection_ip_blocks WHERE id=?`, block.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || expiry != 0 || active != 0 || completed.Receipt.Status != string(providerops.StatusSucceeded) {
		t.Fatalf("completion=%+v expiry=%d active=%d err=%v", completed, expiry, active, err)
	}
}
