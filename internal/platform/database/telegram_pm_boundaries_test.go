package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPMReplyInferencePreservesExactMessageTime(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := pmMember(t, store, 31986)
	admin := createTestAdmin(t, store, 31987)
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	first := queueSuccessfulPMOutbound(t, ctx, store, user, admin, 801, 1, 901, at)
	second := queueSuccessfulPMOutbound(t, ctx, store, user, admin, 802, 2, 902, at.Add(time.Nanosecond))
	if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 803, ChatID: -100123,
		SourceChatID: user.TelegramID, SourceMessageID: 3, MessageAt: at, Inbound: true}, at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	conversation, found, err := store.PMConversationByUser(ctx, user.ID)
	if err != nil || !found {
		t.Fatal("conversation", found, err)
	}
	deliveries, err := store.ListPMDeliveries(ctx, conversation.ID)
	if err != nil || len(deliveries) != 3 {
		t.Fatal("deliveries", len(deliveries), err)
	}
	if deliveries[1].OperationID != second.ID || deliveries[1].ReadAt != nil || deliveries[2].OperationID != first.ID || deliveries[2].ReadAt == nil {
		t.Fatal("future delivery was inferred read or chronology changed", deliveries)
	}
}

func TestPMConversationPagesAndUpdateRetentionPreserveNanoseconds(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := pmMember(t, store, 31988)
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	for i, chat := range []int64{-100123, -100124} {
		if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: int64(811 + i), ChatID: chat,
			SourceChatID: user.TelegramID, SourceMessageID: int64(i + 1), Inbound: true}, at.Add(time.Duration(i)*time.Nanosecond)); err != nil {
			t.Fatal(err)
		}
	}
	current, found, err := store.PMConversationByUser(ctx, user.ID)
	if err != nil || !found || current.ChatID != -100124 {
		t.Fatal("latest route selected an older conversation", current, err)
	}
	page, cursor, err := store.ListPMConversations(ctx, "", "", 1)
	if err != nil || len(page) != 1 || page[0].ChatID != -100124 || cursor == nil {
		t.Fatal("first page", page, cursor, err)
	}
	page, cursor, err = store.ListPMConversations(ctx, *cursor, "", 1)
	if err != nil || len(page) != 1 || page[0].ChatID != -100123 || cursor != nil {
		t.Fatal("second page", page, cursor, err)
	}
	for i, want := range []int64{1, 0} {
		now := at.Add(pmReceiptRetention + time.Duration(i)*time.Nanosecond)
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		var retained int64
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM telegram_pm_updates WHERE update_id IN (811,812)`).Scan(&retained); err != nil || retained != want {
			t.Fatal("update receipt retention", retained, want, err)
		}
	}
}
