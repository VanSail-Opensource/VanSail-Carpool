package telegrampm

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) HandleCallback(ctx context.Context, updateID int64, query *telegram.CallbackQuery) (bool, error) {
	if query == nil || !strings.HasPrefix(query.Data, "pm:") {
		return false, nil
	}
	if query.ID == "" || updateID <= 0 || s.Callbacks == nil {
		return true, ErrInvalidUpdate
	}
	answer := func(reason string, alert bool) error {
		return s.Callbacks.AnswerCallbackQuery(ctx, query.ID, callbackText(query.From.LanguageCode, reason), alert)
	}
	if query.Message == nil || query.From.IsBot {
		return true, answer("denied", true)
	}
	parts := strings.Split(query.Data, ":")
	if len(parts) != 3 {
		return true, answer("denied", true)
	}
	if !s.AdminIDs[query.From.ID] {
		return true, answer("denied", true)
	}
	enabled, group, err := Configuration(ctx, s.Settings)
	if err != nil {
		return true, err
	}
	if !enabled || query.Message.Chat.ID != group || query.Message.MessageThreadID <= 1 {
		return true, answer("denied", true)
	}
	conversation, found, err := s.Repository.PMConversationByTopic(ctx, group, query.Message.MessageThreadID)
	if err != nil {
		return true, err
	}
	if !found || conversation.ID != parts[2] || conversation.ProfileMessageID != query.Message.MessageID {
		return true, answer("denied", true)
	}
	admin, err := s.adminIdentity(ctx, query.From)
	if err != nil {
		return true, err
	}
	if parts[1] == "refresh" {
		_, err := s.Repository.QueuePMProfileRefresh(ctx, admin.ID, "callback:"+strconv.FormatInt(updateID, 10), conversation.ID, updateID, time.Now().UTC())
		if err != nil {
			return true, err
		}
		return true, answer("refresh", false)
	}
	input := model.PMModerationInput{ConversationID: conversation.ID}
	value := parts[1] == "block" || parts[1] == "mute"
	switch parts[1] {
	case "block", "unblock":
		input.Blocked = &value
	case "mute", "unmute":
		input.Muted = &value
	default:
		return true, answer("denied", true)
	}
	if _, err := s.Repository.QueuePMModeration(ctx, admin.ID, "callback:"+strconv.FormatInt(updateID, 10), input, updateID, time.Now().UTC()); err != nil {
		return true, err
	}
	return true, answer("updated", false)
}
