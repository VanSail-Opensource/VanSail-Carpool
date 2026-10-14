package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func newActivityService(store *database.Store, settings *admin.SettingsService, telegramClient *queuedTelegram) *activity.Service {
	service := activity.NewService(store, activity.CryptoRandom{}, nil)
	service.SetGroupBoostSource(activityBoostAdapter{settings: settings, telegram: telegramClient, users: store})
	return service
}

type activityBoostAdapter struct {
	settings interface {
		Optional(context.Context, string) (string, error)
	}
	telegram interface {
		GetUserChatBoosts(context.Context, string, int64) (telegram.UserChatBoosts, error)
	}
	users interface {
		UserByID(context.Context, string) (model.User, error)
	}
}

func (adapter activityBoostAdapter) GroupBoost(ctx context.Context, userID string) (activity.GroupBoostStatus, error) {
	status := activity.GroupBoostStatus{State: "unavailable"}
	configured, err := adapter.settings.Optional(ctx, "telegram.group_chat_id")
	if err != nil {
		return status, err
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(configured), 10, 64)
	// Boosts are supported by supergroups; basic groups cannot satisfy this gate.
	chat := strings.TrimPrefix(strconv.FormatInt(chatID, 10), "-100")
	if err != nil || chatID >= 0 || !strings.HasPrefix(strconv.FormatInt(chatID, 10), "-100") || chat == "" || chat == "0" {
		return status, fmt.Errorf("configured Telegram group is not a supergroup")
	}
	link := "https://t.me/boost?c=" + chat
	status.BoostURL = &link
	user, err := adapter.users.UserByID(ctx, userID)
	if err != nil {
		return status, err
	}
	boosts, err := adapter.telegram.GetUserChatBoosts(ctx, strconv.FormatInt(chatID, 10), user.TelegramID)
	if err != nil {
		return status, err
	}
	seen := make(map[string]struct{}, len(boosts.Boosts))
	now := time.Now().Unix()
	for _, boost := range boosts.Boosts {
		if boost.BoostID != "" && boost.AddDate > 0 && boost.AddDate <= now && boost.ExpirationDate > now {
			seen[boost.BoostID] = struct{}{}
		}
	}
	count := len(seen)
	status.State, status.Count = "required", &count
	if count > 0 {
		status.State = "boosted"
	}
	return status, nil
}
