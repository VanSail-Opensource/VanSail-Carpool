package httpapi

import (
	"context"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/botcommands"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func (s *Server) processTelegramMembershipUpdate(ctx context.Context, requestID string, update *telegram.ChatMemberUpdated) {
	if update == nil {
		return
	}
	member := update.NewChatMember.User
	if _, err := s.deps.Accounts.RefreshMembershipByTelegramID(ctx, member.ID); err != nil {
		s.deps.Logger.Warn("Telegram membership refresh failed", "request_id", requestID,
			"chat_id", update.Chat.ID, "telegram_id", member.ID, "error", err)
	}
	groupID, configured := s.telegramGroupID(ctx)
	if !configured || !telegramMembershipJoined(update, groupID) {
		return
	}
	copy := botcommands.Text(botcommands.LanguageFor(member.LanguageCode))
	message := botcommands.FormatWelcome(copy, member.ID, telegramMemberDisplayName(member))
	if err := s.deps.Telegram.SendMarkdownV2Message(ctx, update.Chat.ID, 0, message); err != nil {
		s.deps.Logger.Warn("send Telegram member welcome", "request_id", requestID,
			"chat_id", update.Chat.ID, "telegram_id", member.ID, "error", err)
	}
}

func telegramMembershipJoined(update *telegram.ChatMemberUpdated, configuredGroupID int64) bool {
	if update == nil || configuredGroupID == 0 || update.Chat.ID != configuredGroupID {
		return false
	}
	return !update.NewChatMember.User.IsBot && !update.OldChatMember.Present() && update.NewChatMember.Present()
}

func telegramMemberDisplayName(user telegram.User) string {
	if name := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName)); name != "" {
		return name
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		return "@" + strings.TrimLeft(username, "@")
	}
	return ""
}
