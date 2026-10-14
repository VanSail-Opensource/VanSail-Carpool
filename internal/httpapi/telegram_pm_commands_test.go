package httpapi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestTelegramPMCommandTargetRequiresConfiguredAdminAndUserTopic(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "pm-command-target.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := database.NewStore(db)
	member, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 54801, FirstName: "Member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterPanelEntry(ctx, member.TelegramID, false); err != nil {
		t.Fatal(err)
	}
	adminUser, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 54802, FirstName: "Admin"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: member.ID, UserID: member.ID, UpdateID: 901,
		ChatID: -100548, SourceChatID: member.TelegramID, SourceMessageID: 1, Inbound: true}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	conversation, found, err := store.PMConversationByUser(ctx, member.ID)
	if err != nil || !found {
		t.Fatalf("conversation found=%v err=%v", found, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE telegram_pm_conversations SET topic_id=55,topic_state='ready' WHERE id=?`, conversation.ID); err != nil {
		t.Fatal(err)
	}
	conversation.TopicID = 55
	server := &Server{deps: Dependencies{Store: store}, adminTelegramIDs: map[int64]struct{}{adminUser.TelegramID: {}}}
	message := &telegram.Message{MessageThreadID: conversation.TopicID, From: &telegram.User{ID: adminUser.TelegramID}, Chat: telegram.Chat{ID: conversation.ChatID, Type: "supergroup"}}
	gotConversation, gotAdmin, ok := server.telegramPMCommandTarget(ctx, message)
	if !ok || gotConversation.ID != conversation.ID || gotAdmin.ID != adminUser.ID {
		t.Fatalf("valid target = (%+v,%+v,%v)", gotConversation, gotAdmin, ok)
	}
	message.MessageThreadID++
	if _, _, ok := server.telegramPMCommandTarget(ctx, message); ok {
		t.Fatal("command resolved outside the member's topic")
	}
	message.MessageThreadID = conversation.TopicID
	message.From.ID = member.TelegramID
	if _, _, ok := server.telegramPMCommandTarget(ctx, message); ok {
		t.Fatal("non-admin resolved as a PM command actor")
	}
}

func TestPMRefundRequiresPositiveTrafficQuote(t *testing.T) {
	t.Parallel()
	zero := model.TXBMoney(0)
	positive := model.TXBMoney(1250)
	for _, test := range []struct {
		name  string
		quote admin.EntitlementRefundQuote
		want  int64
		ok    bool
	}{
		{name: "unavailable"},
		{name: "zero", quote: admin.EntitlementRefundQuote{SuggestedRefund: &zero}},
		{name: "positive", quote: admin.EntitlementRefundQuote{SuggestedRefund: &positive}, want: 1250, ok: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := pmRefundAmount(test.quote)
			if got != test.want || ok != test.ok {
				t.Fatalf("pmRefundAmount() = (%d,%t), want (%d,%t)", got, ok, test.want, test.ok)
			}
		})
	}
}
