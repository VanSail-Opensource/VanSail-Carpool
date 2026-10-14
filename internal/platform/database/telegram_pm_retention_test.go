package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPMRetentionBoundsReceiptsAndKeepsUncertainRoutingAndOwners(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	user := pmMember(t, store, 31926)
	old := now.Add(-pmReceiptRetention)
	receipt := queuedPM(t, store, user, 401, 1, old)
	items, _, err := store.ListPMConversations(ctx, "", "", 25)
	if err != nil {
		t.Fatal(err)
	}
	id := items[0].ID
	if _, err := store.BeginProviderOperationAttempt(ctx, receipt.ID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimPMTopic(ctx, id, receipt.ID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, receipt.ID, "topic", old); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET created_at=? WHERE id=?`, stamp(old), user.ID); err != nil {
		t.Fatal(err)
	}
	recent := queuedPM(t, store, user, 402, 2, now.Add(-time.Hour))
	if _, err := store.BeginProviderOperationAttempt(ctx, recent.ID, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, recent.ID, providerops.Completion{Status: providerops.StatusSucceeded}, now); err != nil {
		t.Fatal(err)
	}
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["telegram_pm_updates"] != 1 || counts["telegram_pm_topic_reviews"] != 1 {
		t.Fatalf("counts=%+v", counts)
	}
	conversation, err := store.PMConversation(ctx, id)
	if err != nil || conversation.TopicState != "pending_review" {
		t.Fatalf("uncertainty=%+v,%v", conversation, err)
	}
	if _, err := store.ProviderOperationByID(ctx, recent.ID); err != nil {
		t.Fatalf("recent delivery deleted: %v", err)
	}
	if _, err := store.UserByID(ctx, user.ID); err != nil {
		t.Fatalf("conversation owner pruned: %v", err)
	}
	var failed int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&failed); err != nil || failed != 0 {
		t.Fatalf("FK errors=%d,%v", failed, err)
	}
}
