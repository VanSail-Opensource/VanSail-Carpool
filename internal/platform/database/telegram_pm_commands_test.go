package database

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func pmMember(t *testing.T, store *Store, id int64) model.User {
	t.Helper()
	user := createTestUser(t, store, id)
	if err := store.RegisterPanelEntry(context.Background(), id, false); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestPMConcurrentWebhookReplayQueuesOneReferenceOnlyOperation(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := pmMember(t, store, 31920)
	now := time.Now().UTC()
	input := model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 100, ChatID: -100123, SourceChatID: user.TelegramID, SourceMessageID: 45, Inbound: true}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if _, err := store.QueuePMRelay(ctx, input, now); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	var operations, updates, conversations int
	for _, query := range []struct {
		sql   string
		count *int
	}{{`SELECT COUNT(*) FROM provider_operations WHERE kind='telegram_pm_relay'`, &operations}, {`SELECT COUNT(*) FROM telegram_pm_updates`, &updates}, {`SELECT COUNT(*) FROM telegram_pm_conversations`, &conversations}} {
		if err := store.DB().QueryRowContext(ctx, query.sql).Scan(query.count); err != nil {
			t.Fatal(err)
		}
	}
	if operations != 1 || updates != 1 || conversations != 1 {
		t.Fatalf("counts=%d,%d,%d", operations, updates, conversations)
	}
	var payload, target string
	if err := store.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT target_id FROM provider_operation_items WHERE item_key='relay'`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if target != "31920:45" || strings.Contains(payload, "text") || strings.Contains(payload, "caption") || strings.Contains(payload, "file_id") {
		t.Fatalf("stored reference=%q payload=%q", target, payload)
	}
	if conflict, err := store.ComboControlConflict(ctx, user.ID); err != nil || conflict {
		t.Fatalf("PM blocks combo controls=%v,%v", conflict, err)
	}
}

func TestPMModerationIsIdempotentAndCarriesAcrossDestinationGroups(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := pmMember(t, store, 31921)
	admin := createTestUser(t, store, 31922)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	input := model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 101, ChatID: -100123, SourceChatID: user.TelegramID, SourceMessageID: 1, Inbound: true}
	if _, err := store.QueuePMRelay(ctx, input, now); err != nil {
		t.Fatal(err)
	}
	items, _, err := store.ListPMConversations(ctx, "", "", 25)
	if err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	muted := true
	command := model.PMModerationInput{ConversationID: items[0].ID, Muted: &muted}
	first, err := store.QueuePMModeration(ctx, admin.ID, "mute", command, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.QueuePMModeration(ctx, admin.ID, "mute", command, 0, now)
	if err != nil || first.ID != replay.ID {
		t.Fatalf("replay=%+v,%v", replay, err)
	}
	input.UpdateID, input.ChatID = 102, -100124
	if _, err := store.QueuePMRelay(ctx, input, now); err != nil {
		t.Fatal(err)
	}
	items, _, err = store.ListPMConversations(ctx, "", "", 25)
	if err != nil || len(items) != 2 {
		t.Fatal(err)
	}
	for _, item := range items {
		if !item.Muted {
			t.Fatal("destination change lost normalized mute flag")
		}
	}
	blocked := true
	if _, err := store.QueuePMModeration(ctx, admin.ID, "block", model.PMModerationInput{ConversationID: items[0].ID, Blocked: &blocked}, 0, now); err != nil {
		t.Fatal(err)
	}
	input.UpdateID = 103
	if _, err := store.QueuePMRelay(ctx, input, now); err == nil {
		t.Fatal("blocked sender queued")
	}
}

func TestPMProfileRefreshQueuesOneScopedOperation(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := pmMember(t, store, 31984)
	admin := createTestUser(t, store, 31985)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 701, ChatID: -100123,
		SourceChatID: user.TelegramID, SourceMessageID: 9, Inbound: true}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	conversation, found, err := store.PMConversationByUser(ctx, user.ID)
	if err != nil || !found {
		t.Fatalf("conversation found=%v err=%v", found, err)
	}
	now := time.Now().UTC()
	first, err := store.QueuePMProfileRefresh(ctx, admin.ID, "refresh-701", conversation.ID, 702, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.QueuePMProfileRefresh(ctx, admin.ID, "refresh-701", conversation.ID, 702, now.Add(time.Second))
	if err != nil || replay.ID != first.ID {
		t.Fatalf("refresh replay = %+v, %v", replay, err)
	}
	if _, err := store.QueuePMProfileRefresh(ctx, user.ID, "refresh-user", conversation.ID, 703, now); err == nil {
		t.Fatal("member queued an administrator profile refresh")
	}
}
