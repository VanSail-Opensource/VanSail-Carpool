package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestCompactAndPrunePreservesReferencedProviderOperations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	actor, user := createTestUser(t, store, 50_401), createTestUser(t, store, 50_402)
	now := time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC)

	unreferenced, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: actor.ID, OwnerUserID: user.ID, Kind: "maintenance-unreferenced",
		IdempotencyKey: "unreferenced-key", RequestFingerprint: "unreferenced-fingerprint",
		Items: []providerops.ItemInput{{Key: user.ID, TargetType: "user", TargetID: user.ID}},
	}, now.Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, unreferenced.Receipt.ID, now.Add(-47*time.Hour))

	ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: user.ID,
		IdempotencyKey: "maintenance-ban-key", RequestFingerprint: "maintenance-ban-fingerprint", Reason: "retention check", DurationMinutes: 1}, now.Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, ban.ID, now.Add(-47*time.Hour))
	unban, err := store.CreateAdminTemporaryUnban(ctx, actor.ID, user.ID, "maintenance-unban-key", "maintenance-unban-fingerprint", "retention check", now.Add(-47*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, unban.ID, now.Add(-46*time.Hour))

	compensation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: actor.ID, OwnerUserID: user.ID, Kind: "maintenance-compensation",
		IdempotencyKey: "compensation-key", RequestFingerprint: "compensation-fingerprint",
		Items: []providerops.ItemInput{{Key: user.ID, TargetType: "user", TargetID: user.ID}},
	}, now.Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, compensation.Receipt.ID, now.Add(-47*time.Hour))
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO node_compensation_events(
		id,node_uuid,node_name,status,offline_observed_at,threshold_minutes,multiplier_bps,
		frozen_recipient_count,provider_operation_id,revision,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, "maintenance-event", "maintenance-node", "Maintenance node", "queued",
		stamp(now.Add(-48*time.Hour)), 1, 100, 1, compensation.Receipt.ID, 0,
		stamp(now.Add(-48*time.Hour)), stamp(now.Add(-48*time.Hour))); err != nil {
		t.Fatal(err)
	}

	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("CompactAndPrune(): %v", err)
	}
	if counts["provider_operations"] != 1 {
		t.Fatalf("pruned provider operations = %d, want 1", counts["provider_operations"])
	}
	var remaining int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE id IN (?,?,?)`, ban.ID, unban.ID, compensation.Receipt.ID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 3 {
		t.Fatalf("referenced provider operations = %d, want 3", remaining)
	}
}

func completeTestProviderOperation(t *testing.T, store *Store, operationID string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.BeginProviderOperationAttempt(ctx, operationID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, operationID, providerops.Completion{Status: providerops.StatusSucceeded}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}
