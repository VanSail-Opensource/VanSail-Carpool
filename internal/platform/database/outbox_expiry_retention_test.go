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

func TestRestrictionExpiryJobsSurviveCleanupUntilRestoration(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	old := now.Add(-10 * 24 * time.Hour)
	owner := createTestUser(t, store, 497712)
	digest := strings.Repeat("e", 64)
	block, operation, _, err := store.BeginConnectionIPBlock(ctx, connections.CreateIPBlockInput{UserID: owner.ID,
		NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee", IPDigest: digest, SealedIP: "encrypted-ip", ExpiresAt: old.Add(time.Hour)},
		providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: connections.BlockOperationKind,
			IdempotencyKey: "protected-expiry", RequestFingerprint: "protected-expiry-proof",
			Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: digest}}}, old)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, operation.Receipt.ID, old)
	ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: owner.ID, UserID: owner.ID,
		Reason: "temporary restriction", DurationMinutes: 60, IdempotencyKey: "protected-ban", RequestFingerprint: "protected-ban-proof"}, old)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, ban.ID, old)
	var banJobID string
	if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind='admin_temporary_ban_expiry'`).Scan(&banJobID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{block.ExpiryJobID, banJobID} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10,updated_at=$1 WHERE id=$2`, stamp(old), id); err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteOutboxJob(ctx, id, owner.ID); !errors.Is(err, ErrConflict) {
			t.Fatal("expiry continuation deleted before restoration", id, err)
		}
	}
	for range 2 {
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		var retained int
		if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE id IN ($1,$2)`, block.ExpiryJobID, banJobID).Scan(&retained); err != nil || retained != 2 {
			t.Fatal("failed delivery was mistaken for confirmed restoration", retained, err)
		}
	}
	if err := store.FinalizeConnectionIPBlockExpiry(ctx, block.ID, true, now); err != nil {
		t.Fatal(err)
	}
	unban, err := store.CreateAdminTemporaryUnban(ctx, owner.ID, owner.ID, "confirmed-unban", "confirmed-unban-proof", "restored", now)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, unban.ID, now)
	if err := store.CompleteAdminTemporaryUnban(ctx, owner.ID, unban.ID, now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{block.ExpiryJobID, banJobID} {
		if err := store.DeleteOutboxJob(ctx, id, owner.ID); err != nil {
			t.Fatal("restored restriction kept its obsolete expiry artifact", id, err)
		}
	}
}
