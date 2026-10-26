package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestFailedSynchronizationRetentionUsesFinalFailureAndKeepsFinancialWork(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		id, kind, status string
		failed           time.Time
	}{
		{"expired", "xnode_sync_user", "failed", now.Add(-failedSyncRetention)},
		{"expired-payg", "xnode_payg_reconcile", "failed", now.Add(-failedSyncRetention)},
		{"recent-failure", "xnode_sync_user", "failed", now.Add(-failedSyncRetention + time.Nanosecond)},
		{"pending", "xnode_apply_entitlement", "pending", now.Add(-8 * 24 * time.Hour)},
		{"processing", "xnode_sync_user", "processing", now.Add(-8 * 24 * time.Hour)},
		{"financial", "purchase_expire", "failed", now.Add(-8 * 24 * time.Hour)},
		{"raffle-financial", "draw_raffle_settle", "failed", now.Add(-8 * 24 * time.Hour)},
	} {
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES(?,?,?, ?,10,?,?,?)`, test.id, test.kind, `{"userId":"missing"}`, test.status, stamp(now), stamp(now.Add(-10*24*time.Hour)), stamp(test.failed)); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["failed_synchronization_jobs"] != 2 {
		t.Fatalf("counts=%+v", counts)
	}
	var remaining int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs`).Scan(&remaining); err != nil || remaining != 5 {
		t.Fatalf("remaining=%d,%v", remaining, err)
	}
}

func TestFailedSyncCleanupKeepsPurchaseMoneyAndProtectsQueuedManualRetry(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 31933)
	actor := createTestUser(t, store, 31934)
	combo := saveTestCombo(t, store, "failed-sync-ownership", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "retention-sync-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "retention-sync-purchase"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10,updated_at=? WHERE kind='xnode_apply_entitlement' AND json_extract(payload,'$.purchaseId')=?`, stamp(now.Add(-failedSyncRetention)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES('retry-target','xnode_sync_user','{"userId":"missing"}','failed',10,?,?,?)`, stamp(now), stamp(now.Add(-8*24*time.Hour)), stamp(now.Add(-failedSyncRetention))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID, Kind: providerops.KindOutboxRetry, IdempotencyKey: "retained-retry", RequestFingerprint: "retained-retry-fingerprint", Items: []providerops.ItemInput{{Key: "job", TargetType: "outbox_job", TargetID: "retry-target"}}}, now); err != nil {
		t.Fatal(err)
	}
	before, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["failed_synchronization_jobs"] != 1 {
		t.Fatalf("counts=%+v", counts)
	}
	updated, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || updated.Status != "failed" || updated.PriceTXBMinor != purchase.PriceTXBMinor {
		t.Fatalf("purchase=%+v,%v", updated, err)
	}
	after, err := store.Balance(ctx, user.ID)
	if err != nil || after.Minor != before.Minor {
		t.Fatalf("balance changed=%+v,%v", after, err)
	}
	var retained int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE id='retry-target'`).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("manual retry target removed=%d,%v", retained, err)
	}
	if _, err := store.PurchaseByID(ctx, purchase.ID); errors.Is(err, ErrNotFound) {
		t.Fatal("purchased ownership erased")
	}
}

func TestFailedPMJobExpiryKeepsUncertainTopicAndNeverAttemptedRelay(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	user := pmMember(t, store, 31935)
	receipt := queuedPM(t, store, user, 501, 5, now)
	conversations, _, err := store.ListPMConversations(ctx, "", "", 25)
	if err != nil {
		t.Fatal(err)
	}
	id := conversations[0].ID
	if _, err := store.BeginProviderOperationAttempt(ctx, receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, receipt.ID, "topic", now); err != nil {
		t.Fatal(err)
	}
	if claimed, err := store.ClaimPMTopic(ctx, id, receipt.ID, now); err != nil || !claimed {
		t.Fatalf("claim=%v,%v", claimed, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10,updated_at=? WHERE kind='provider_operation' AND json_extract(payload,'$.operationId')=?`, stamp(now.Add(-failedSyncRetention)), receipt.ID); err != nil {
		t.Fatal(err)
	}
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["failed_synchronization_jobs"] != 1 {
		t.Fatalf("counts=%+v", counts)
	}
	conversation, err := store.PMConversation(ctx, id)
	if err != nil || conversation.TopicState != "pending_review" {
		t.Fatalf("topic=%+v,%v", conversation, err)
	}
	operation, err := store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil || operation.Receipt.Status != "pending_review" || operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != "PM_TOPIC_UNCERTAIN" {
		t.Fatalf("receipt=%+v,%v", operation, err)
	}
	items, err := store.ProviderOperationItems(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Key == "relay" && (item.Status != providerops.StatusQueued || item.AttemptStartedAt != nil) {
			t.Fatal("never-sent relay lost safe recovery state")
		}
	}
}
