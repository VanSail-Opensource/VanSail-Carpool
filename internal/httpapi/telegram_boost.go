package httpapi

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Server) processTelegramBoost(ctx context.Context, update *telegram.ChatBoostUpdated) error {
	configured, err := s.deps.Settings.Optional(ctx, "telegram.group_chat_id")
	if err != nil {
		return err
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(configured), 10, 64)
	if err != nil || chatID != update.Chat.ID {
		return nil
	}
	now := time.Now().UTC()
	boost := update.Boost
	// The authenticated update is authoritative for when the boost was added.
	// Comparing its second-resolution timestamp with the local clock can drop
	// a new boost when the server clock is slightly behind Telegram's.
	if boost.BoostID == "" || boost.AddDate <= 0 || boost.ExpirationDate <= now.Unix() {
		return nil
	}
	item := model.TelegramBoostAppreciation{ChatID: chatID, BoostID: boost.BoostID}
	if user := boost.Source.User; user != nil {
		item.Username = user.Username
		item.Name = strings.TrimSpace(user.FirstName + " " + user.LastName)
		item.Locale = user.LanguageCode
	}
	_, err = s.deps.Store.EnqueueBoostAppreciation(ctx, item, now)
	return err
}
