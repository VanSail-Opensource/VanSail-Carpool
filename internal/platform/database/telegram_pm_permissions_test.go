package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPMNormalAccountCannotModerateOrQueueForumReplies(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := pmMember(t, store, 31923)
	input := model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: 201, ChatID: -100123, SourceChatID: user.TelegramID, SourceMessageID: 2, Inbound: true}
	if _, err := store.QueuePMRelay(ctx, input, now); err != nil {
		t.Fatal(err)
	}
	items, _, err := store.ListPMConversations(ctx, "", "", 25)
	if err != nil {
		t.Fatal(err)
	}
	blocked := true
	if _, err := store.QueuePMModeration(ctx, user.ID, "invalid-moderator", model.PMModerationInput{ConversationID: items[0].ID, Blocked: &blocked}, 0, now); err == nil {
		t.Fatal("normal account moderated")
	}
	input.UpdateID, input.SourceChatID, input.Inbound = 202, -100123, false
	if _, err := store.QueuePMRelay(ctx, input, now); err == nil {
		t.Fatal("normal account relayed forum reply")
	}
	if _, err := store.QueuePMTopicRepair(ctx, user.ID, "invalid-repair", model.PMTopicRepairInput{ConversationID: items[0].ID, TopicID: 70}, now); err == nil {
		t.Fatal("normal account repaired topic")
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM telegram_pm_updates WHERE update_id=202`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected request consumed receipt=%d,%v", count, err)
	}
}
