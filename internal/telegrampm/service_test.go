package telegrampm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type serviceRepo struct {
	ServiceRepository
	qualified, blocked, muted bool
	queued                    []model.PMRelayInput
	notices                   []model.PMNotice
	moderation                []model.PMModerationInput
	conversation              model.PMConversation
}

func (r *serviceRepo) PanelEntryQualified(context.Context, int64) (bool, error) {
	return r.qualified, nil
}
func (r *serviceRepo) UserByTelegramID(context.Context, int64) (model.User, error) {
	return model.User{ID: "member", TelegramID: 42}, nil
}
func (r *serviceRepo) UpsertTelegramUser(_ context.Context, p model.TelegramProfile, admin bool) (model.User, bool, error) {
	id, role := "member", "user"
	if admin {
		id, role = "admin", "admin"
	}
	return model.User{ID: id, TelegramID: p.ID, Role: role}, false, nil
}
func (r *serviceRepo) PMUserFlags(context.Context, string) (bool, bool, error) {
	return r.blocked, r.muted, nil
}
func (r *serviceRepo) PMConversationByTopic(_ context.Context, chat, topic int64) (model.PMConversation, bool, error) {
	return r.conversation, r.conversation.ChatID == chat && r.conversation.TopicID == topic, nil
}
func (r *serviceRepo) QueuePMNotice(_ context.Context, _ int64, item model.PMNotice, _ time.Time) error {
	r.notices = append(r.notices, item)
	return nil
}
func (r *serviceRepo) QueuePMRelay(_ context.Context, item model.PMRelayInput, _ time.Time) (*model.OperationReceipt, error) {
	r.queued = append(r.queued, item)
	return &model.OperationReceipt{}, nil
}
func (r *serviceRepo) QueuePMModeration(_ context.Context, _, _ string, item model.PMModerationInput, _ int64, _ time.Time) (model.OperationReceipt, error) {
	r.moderation = append(r.moderation, item)
	return model.OperationReceipt{}, nil
}
func (*serviceRepo) QueuePMProfileRefresh(context.Context, string, string, string, int64, time.Time) (model.OperationReceipt, error) {
	return model.OperationReceipt{}, nil
}

type pmSettings struct {
	enabled bool
	group   string
}

func (s pmSettings) Optional(_ context.Context, key string) (string, error) {
	if key == "telegram.pm.enabled" {
		if s.enabled {
			return "true", nil
		}
		return "false", nil
	}
	return s.group, nil
}

type callbackCapture struct {
	text  string
	alert bool
	calls int
}

func (c *callbackCapture) AnswerCallbackQuery(_ context.Context, _, text string, alert bool) error {
	c.text, c.alert = text, alert
	c.calls++
	return nil
}

func privateMessage() *telegram.Message {
	return &telegram.Message{MessageID: 10, From: &telegram.User{ID: 42, FirstName: "Member"}, Chat: telegram.Chat{ID: 42, Type: "private"}, Text: "ordinary **text**"}
}

func TestPrivateRelayRequiresPanelEntryAndPreservesCommandPaths(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                               string
		qualified, enabled, blocked, muted bool
		reason                             string
		count                              int
	}{
		{"new identity", false, true, false, false, "entry", 0},
		{"verification incomplete while disabled", false, false, false, false, "entry", 0},
		{"disabled", true, false, false, false, "disabled", 0},
		{"blocked", true, true, true, false, "blocked", 0},
		{"ordinary text", true, true, false, false, "", 1},
		{"mute keeps relaying", true, true, false, true, "", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := &serviceRepo{qualified: test.qualified, blocked: test.blocked, muted: test.muted}
			s := Service{Repository: r, Settings: pmSettings{enabled: test.enabled, group: "-100123"}}
			if handled, err := s.HandleMessage(context.Background(), 1, privateMessage()); err != nil || !handled {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
			if len(r.queued) != test.count {
				t.Fatalf("relays=%d", len(r.queued))
			}
			if test.reason != "" && (len(r.notices) != 1 || r.notices[0].Reason != test.reason) {
				t.Fatalf("notices=%+v", r.notices)
			}
		})
	}
	r := &serviceRepo{qualified: true}
	s := Service{Repository: r, Settings: pmSettings{enabled: true, group: "-100123"}}
	for _, message := range []*telegram.Message{nil, {MessageID: 1, From: &telegram.User{ID: 42}, Chat: telegram.Chat{ID: 42, Type: "private"}, Text: "/start"}, {MessageID: 2, From: &telegram.User{ID: 42}, SuccessfulPayment: &telegram.SuccessfulPayment{}}, {MessageID: 3, From: &telegram.User{ID: 42, IsBot: true}, Text: "hello"}} {
		if handled, err := s.HandleMessage(context.Background(), 2, message); handled || err != nil {
			t.Fatalf("existing path intercepted: %v,%v", handled, err)
		}
	}
}

func TestPMMediaUsesReferencesAndRejectsUncopyableQuiz(t *testing.T) {
	t.Parallel()
	r := &serviceRepo{qualified: true}
	s := Service{Repository: r, Settings: pmSettings{enabled: true, group: "-100123"}}
	message := privateMessage()
	message.Text = ""
	message.Photo = json.RawMessage(`[{"file_id":"upstream-only"}]`)
	if handled, err := s.HandleMessage(context.Background(), 3, message); !handled || err != nil || len(r.queued) != 1 {
		t.Fatalf("photo: %v,%v", handled, err)
	}
	if r.queued[0].SourceMessageID != 10 || r.queued[0].SourceChatID != 42 || !r.queued[0].Inbound {
		t.Fatalf("refs=%+v", r.queued[0])
	}
	message.Photo = nil
	message.Poll = &telegram.RelayPoll{Type: "quiz"}
	if _, err := s.HandleMessage(context.Background(), 4, message); err != nil || len(r.queued) != 1 || r.notices[0].Reason != "unsupported" {
		t.Fatalf("quiz=%+v err=%v", r.notices, err)
	}
	r.qualified = false
	if _, err := s.HandleMessage(context.Background(), 5, message); err != nil || r.notices[1].Reason != "entry" {
		t.Fatal("unqualified media bypassed guidance")
	}
}

func TestPMForumRepliesAndCallbacksUsePanelAdminsAndCurrentCard(t *testing.T) {
	t.Parallel()
	r := &serviceRepo{qualified: true, conversation: model.PMConversation{ID: "conversation", UserID: "member", ChatID: -100123, TopicID: 50, ProfileMessageID: 51}}
	capture := &callbackCapture{}
	s := Service{Repository: r, Settings: pmSettings{enabled: true, group: "-100123"}, AdminIDs: map[int64]bool{99: true}, Callbacks: capture}
	message := privateMessage()
	message.Chat = telegram.Chat{ID: -100123, Type: "supergroup"}
	message.MessageThreadID = 50
	if _, err := s.HandleMessage(context.Background(), 6, message); err != nil || len(r.queued) != 0 {
		t.Fatal("forum membership granted reply permission")
	}
	message.From.ID = 99
	if _, err := s.HandleMessage(context.Background(), 7, message); err != nil || len(r.queued) != 1 || r.queued[0].Inbound || r.queued[0].ActorUserID != "admin" {
		t.Fatalf("admin relay=%+v,%v", r.queued, err)
	}
	query := &telegram.CallbackQuery{ID: "query", From: telegram.User{ID: 42}, Message: &telegram.Message{MessageID: 51, MessageThreadID: 50, Chat: message.Chat}, Data: "pm:block:conversation"}
	if _, err := s.HandleCallback(context.Background(), 8, query); err != nil || !capture.alert || len(r.moderation) != 0 {
		t.Fatal("non-admin callback accepted")
	}
	query.From.ID = 99
	query.Message.MessageID = 52
	if _, err := s.HandleCallback(context.Background(), 9, query); err != nil || len(r.moderation) != 0 {
		t.Fatal("stale profile card accepted")
	}
	query.Message.MessageID = 51
	if _, err := s.HandleCallback(context.Background(), 10, query); err != nil || len(r.moderation) != 1 || r.moderation[0].Blocked == nil || !*r.moderation[0].Blocked || capture.alert {
		t.Fatalf("moderation=%+v,%v", r.moderation, err)
	}
	query.Data = "pm:block:another-conversation"
	if _, err := s.HandleCallback(context.Background(), 11, query); err != nil || len(r.moderation) != 1 {
		t.Fatal("cross-conversation callback accepted")
	}
	query.ID = ""
	query.Data = "pm:block:conversation"
	if _, err := s.HandleCallback(context.Background(), 12, query); err == nil || len(r.moderation) != 1 {
		t.Fatal("invalid callback envelope mutated flags")
	}
}
