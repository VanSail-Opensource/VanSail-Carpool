package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func queuedPM(t *testing.T, store *Store, user model.User, update, message int64, now time.Time) model.OperationReceipt {
	t.Helper()
	receipt, err := store.QueuePMRelay(context.Background(), model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: update, ChatID: -100123, SourceChatID: user.TelegramID, SourceMessageID: message, Inbound: true}, now)
	if err != nil || receipt == nil {
		t.Fatalf("queue=%+v,%v", receipt, err)
	}
	return *receipt
}

func TestPMTopicAndProfileClaimsSerializeAcrossOperations(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := pmMember(t, store, 31924)
	first, second := queuedPM(t, store, user, 301, 1, now), queuedPM(t, store, user, 302, 2, now)
	conversations, _, err := store.ListPMConversations(ctx, "", "", 25)
	if err != nil {
		t.Fatal(err)
	}
	id := conversations[0].ID
	if claimed, err := store.ClaimPMTopic(ctx, id, first.ID, now); !claimed || err != nil {
		t.Fatalf("first claim=%v,%v", claimed, err)
	}
	if claimed, err := store.ClaimPMTopic(ctx, id, second.ID, now); claimed || err != nil {
		t.Fatalf("second claim=%v,%v", claimed, err)
	}
	if err := store.SavePMTopic(ctx, id, first.ID, 50, now); err != nil {
		t.Fatal(err)
	}
	for _, receipt := range []model.OperationReceipt{first, second} {
		if _, err := store.BeginProviderOperationAttempt(ctx, receipt.ID, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, first.ID, "profile", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, second.ID, "profile", now); err == nil {
		t.Fatal("concurrent profile publication not serialized")
	}
	if err := store.SavePMProfile(ctx, id, 50, 51, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperationItem(ctx, first.ID, "profile", providerops.Completion{Status: providerops.StatusSucceeded, ProviderReference: "51"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, second.ID, "profile", now); err != nil {
		t.Fatalf("completed profile lock not released: %v", err)
	}
	conversation, err := store.PMConversation(ctx, id)
	if err != nil || conversation.TopicID != 50 || conversation.ProfileMessageID != 51 || conversation.ProfileState != "ready" {
		t.Fatalf("references=%+v,%v", conversation, err)
	}
}

func TestPMRepairResumesOnlyNeverAttemptedRelay(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := pmMember(t, store, 31925)
	waiter := queuedPM(t, store, user, 303, 3, now)
	conversations, _, err := store.ListPMConversations(ctx, "", "", 25)
	if err != nil {
		t.Fatal(err)
	}
	id := conversations[0].ID
	if _, err := store.BeginProviderOperationAttempt(ctx, waiter.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, waiter.ID, "topic", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimPMTopic(ctx, id, waiter.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleasePMTopic(ctx, id, waiter.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperationItem(ctx, waiter.ID, "topic", providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "PM_TOPIC_UNCERTAIN"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, waiter.ID, providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "PM_TOPIC_UNCERTAIN"}, now); err != nil {
		t.Fatal(err)
	}
	ambiguous := queuedPM(t, store, user, 304, 4, now)
	if _, err := store.BeginProviderOperationAttempt(ctx, ambiguous.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, ambiguous.ID, "relay", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperationItem(ctx, ambiguous.ID, "relay", providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "PM_DELIVERY_UNCERTAIN"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, ambiguous.ID, providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "PM_DELIVERY_UNCERTAIN"}, now); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePMTopicRepair(ctx, id, waiter.ID, 90, 91, now); err != nil {
		t.Fatal(err)
	}
	resumed, err := store.ProviderOperationByID(ctx, waiter.ID)
	if err != nil || resumed.Receipt.Status != "queued" {
		t.Fatalf("waiter=%+v,%v", resumed, err)
	}
	untouched, err := store.ProviderOperationByID(ctx, ambiguous.ID)
	if err != nil || untouched.Receipt.Status != "pending_review" {
		t.Fatalf("ambiguous replayed=%+v,%v", untouched, err)
	}
}
