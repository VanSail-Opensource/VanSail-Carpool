package httpapi

import (
	"context"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func (s *Server) processTelegramBotStatus(ctx context.Context, update telegram.Update) error {
	change := update.MyChatMember
	if change == nil || change.Chat.Type != "private" || change.Chat.ID <= 0 || !change.NewChatMember.User.IsBot || change.Date <= 0 {
		return nil
	}
	blocked := change.NewChatMember.Status == "kicked"
	if !blocked && !(change.OldChatMember.Status == "kicked" && change.NewChatMember.Status == "member") {
		return nil
	}
	protected := slices.Contains(s.deps.AdminTelegramIDs, change.Chat.ID)
	if user, err := s.deps.Store.UserByTelegramID(ctx, change.Chat.ID); err == nil && user.Role == "admin" {
		protected = true
	}
	if protected && blocked {
		s.deps.Logger.Info("administrator blocked Telegram bot; automatic purge excluded", "telegram_id", change.Chat.ID)
	}
	return s.deps.Store.HandleTelegramBotStatus(ctx, change.Chat.ID, update.UpdateID, change.Date, blocked, protected, time.Now().UTC())
}
