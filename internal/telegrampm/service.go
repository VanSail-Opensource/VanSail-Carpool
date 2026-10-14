// Package telegrampm owns qualified private messaging and panel-admin forum routing.
package telegrampm

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var ErrDisabled = errors.New("PM relay disabled or destination changed")
var ErrBlocked = errors.New("PM sender blocked")
var ErrUnqualified = errors.New("PM sender must open and verify the panel")
var ErrTopicUncertain = errors.New("PM topic requires review")
var ErrInvalidUpdate = errors.New("invalid PM update envelope")

type ServiceRepository interface {
	PanelEntryQualified(context.Context, int64) (bool, error)
	UserByTelegramID(context.Context, int64) (model.User, error)
	UpsertTelegramUser(context.Context, model.TelegramProfile, bool) (model.User, bool, error)
	PMUserFlags(context.Context, string) (bool, bool, error)
	PMConversationByTopic(context.Context, int64, int64) (model.PMConversation, bool, error)
	QueuePMNotice(context.Context, int64, model.PMNotice, time.Time) error
	QueuePMRelay(context.Context, model.PMRelayInput, time.Time) (*model.OperationReceipt, error)
	QueuePMModeration(context.Context, string, string, model.PMModerationInput, int64, time.Time) (model.OperationReceipt, error)
	QueuePMProfileRefresh(context.Context, string, string, string, int64, time.Time) (model.OperationReceipt, error)
}

type Settings interface {
	Optional(context.Context, string) (string, error)
}
type CallbackSender interface {
	AnswerCallbackQuery(context.Context, string, string, bool) error
}

type Service struct {
	Repository ServiceRepository
	Settings   Settings
	Callbacks  CallbackSender
	AdminIDs   map[int64]bool
}

func Configuration(ctx context.Context, settings Settings) (bool, int64, error) {
	enabled, err := settings.Optional(ctx, "telegram.pm.enabled")
	if err != nil {
		return false, 0, err
	}
	if enabled == "" || enabled == "false" {
		return false, 0, nil
	}
	if enabled != "true" {
		return false, 0, ErrDisabled
	}
	group, err := settings.Optional(ctx, "telegram.pm.group_chat_id")
	if err != nil {
		return false, 0, err
	}
	id, err := strconv.ParseInt(group, 10, 64)
	if err != nil || id >= 0 || !strings.HasPrefix(group, "-100") {
		return false, 0, ErrDisabled
	}
	return true, id, nil
}

// HandleMessage leaves commands and all payment/service updates on existing paths.
func (s *Service) HandleMessage(ctx context.Context, updateID int64, message *telegram.Message) (bool, error) {
	if message == nil || message.From == nil || message.From.IsBot || message.MessageID <= 0 || message.SuccessfulPayment != nil || message.RefundedPayment != nil || len(message.Invoice) > 0 || strings.HasPrefix(strings.TrimSpace(message.Text), "/") {
		return false, nil
	}
	if !message.UserContent() {
		return false, nil
	}
	if updateID <= 0 {
		return true, ErrInvalidUpdate
	}
	if message.Chat.Type == "private" && message.Chat.ID == message.From.ID {
		return true, s.receivePrivate(ctx, updateID, message)
	}
	if !message.RelayContent() {
		return false, nil
	}
	if message.MessageThreadID <= 1 {
		return false, nil
	}
	enabled, group, err := Configuration(ctx, s.Settings)
	if err != nil {
		return true, err
	}
	if !enabled || group != message.Chat.ID {
		return false, nil
	}
	if !s.AdminIDs[message.From.ID] {
		return true, nil
	}
	conversation, found, err := s.Repository.PMConversationByTopic(ctx, group, message.MessageThreadID)
	if err != nil || !found {
		return true, err
	}
	admin, err := s.adminIdentity(ctx, *message.From)
	if err != nil {
		return true, err
	}
	content, footer, err := prepareContent(message)
	if err != nil {
		return true, err
	}
	_, err = s.Repository.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: admin.ID, UserID: conversation.UserID, UpdateID: updateID,
		ChatID: group, SourceChatID: message.Chat.ID, SourceMessageID: message.MessageID, Content: content, AttributionReply: footer}, time.Now().UTC())
	return true, err
}

func (s *Service) receivePrivate(ctx context.Context, updateID int64, message *telegram.Message) error {
	qualified, err := s.Repository.PanelEntryQualified(ctx, message.From.ID)
	if err != nil {
		return err
	}
	if !qualified {
		return s.notice(ctx, updateID, message, "entry")
	}
	enabled, group, err := Configuration(ctx, s.Settings)
	if err != nil {
		return err
	}
	if !enabled {
		return s.notice(ctx, updateID, message, "disabled")
	}
	if !message.RelayContent() {
		return s.notice(ctx, updateID, message, "unsupported")
	}
	user, err := s.Repository.UserByTelegramID(ctx, message.From.ID)
	if err != nil {
		return err
	}
	language := message.From.LanguageCode
	if language == "" {
		language = user.NotificationLocale
	}
	user, _, err = s.Repository.UpsertTelegramUser(ctx, model.TelegramProfile{ID: message.From.ID, FirstName: message.From.FirstName, LastName: message.From.LastName, Username: message.From.Username, LanguageCode: language}, s.AdminIDs[message.From.ID])
	if err != nil {
		return err
	}
	blocked, _, err := s.Repository.PMUserFlags(ctx, user.ID)
	if err != nil {
		return err
	}
	if blocked {
		return s.notice(ctx, updateID, message, "blocked")
	}
	_, err = s.Repository.QueuePMRelay(ctx, model.PMRelayInput{ActorUserID: user.ID, UserID: user.ID, UpdateID: updateID,
		ChatID: group, SourceChatID: message.Chat.ID, SourceMessageID: message.MessageID, Inbound: true,
		ReplyToMessageID: replyMessageID(message), MessageAt: pmMessageAt(message)}, time.Now().UTC())
	return err
}

func replyMessageID(message *telegram.Message) int64 {
	if message != nil && message.ReplyToMessage != nil {
		return message.ReplyToMessage.MessageID
	}
	return 0
}

func pmMessageAt(message *telegram.Message) time.Time {
	if message != nil && message.Date > 0 {
		return time.Unix(message.Date, 0).UTC()
	}
	return time.Time{}
}

func (s *Service) notice(ctx context.Context, id int64, message *telegram.Message, reason string) error {
	return s.Repository.QueuePMNotice(ctx, id, model.PMNotice{ChatID: message.Chat.ID, ReplyMessageID: message.MessageID, Locale: message.From.LanguageCode, Reason: reason}, time.Now().UTC())
}

func (s *Service) adminIdentity(ctx context.Context, user telegram.User) (model.User, error) {
	if !s.AdminIDs[user.ID] || user.IsBot {
		return model.User{}, ErrBlocked
	}
	identity, _, err := s.Repository.UpsertTelegramUser(ctx, model.TelegramProfile{ID: user.ID, FirstName: user.FirstName, LastName: user.LastName, Username: user.Username, LanguageCode: user.LanguageCode}, true)
	return identity, err
}
