package database

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPMReadStatusUsesReplyInference(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := pmMember(t, store, 31980)
	admin := createTestUser(t, store, 31981)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 501, ChatID: -100123,
		SourceChatID: user.TelegramID, SourceMessageID: 1, Inbound: true}, now); err != nil {
		t.Fatal(err)
	}
	first := queueSuccessfulPMOutbound(t, ctx, store, user, admin, 502, 10, 900, now.Add(time.Second))
	second := queueSuccessfulPMOutbound(t, ctx, store, user, admin, 505, 11, 901, now.Add(4*time.Second))
	if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 506, ChatID: -100123,
		SourceChatID: user.TelegramID, SourceMessageID: 2, ReplyToMessageID: 901, MessageAt: now.Add(5 * time.Second), Inbound: true}, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	conversation, found, err := store.PMConversationByUser(ctx, user.ID)
	if err != nil || !found {
		t.Fatalf("PM conversation = %+v, %v, found=%v", conversation, err, found)
	}
	items, err := store.ListPMDeliveries(ctx, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	reads := make(map[string]string)
	for _, item := range items {
		if item.ReadAt != nil {
			reads[item.OperationID] = item.ReadSource
		}
	}
	if _, read := reads[first.ID]; read || reads[second.ID] != "reply" {
		t.Fatalf("read evidence = %+v", reads)
	}
}

func queueSuccessfulPMOutbound(t *testing.T, ctx context.Context, store *Store, user, admin model.User, updateID, sourceMessage, deliveredMessage int64, now time.Time) model.OperationReceipt {
	t.Helper()
	receipt, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: admin.ID, UserID: user.ID, UpdateID: updateID, ChatID: -100123,
		SourceChatID: -100123, SourceMessageID: sourceMessage}, now)
	if err != nil || receipt == nil {
		t.Fatalf("queue PM outbound: receipt=%+v err=%v", receipt, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='succeeded' WHERE id=?`, receipt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',provider_reference=? WHERE operation_id=? AND item_key='relay'`,
		strconv.FormatInt(deliveredMessage, 10), receipt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operation_items SET status='succeeded',provider_reference='77' WHERE operation_id=? AND item_key='topic'`, receipt.ID); err != nil {
		t.Fatal(err)
	}
	return *receipt
}
