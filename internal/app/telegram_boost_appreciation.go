package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func registerBoostAppreciationHandler(worker *outbox.Worker, settings *admin.SettingsService, client *queuedTelegram) error {
	return worker.Register("telegram_boost_appreciation", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		var item model.TelegramBoostAppreciation
		if err := json.Unmarshal([]byte(job.Payload), &item); err != nil {
			return fmt.Errorf("decode boost appreciation: %w", err)
		}
		configured, err := settings.Optional(ctx, "telegram.group_chat_id")
		if err != nil {
			return err
		}
		if strings.TrimSpace(configured) != strconv.FormatInt(item.ChatID, 10) {
			return nil
		}
		return client.SendMarkdownV2Message(ctx, item.ChatID, 0, boostAppreciationText(item))
	}))
}

func boostAppreciationText(item model.TelegramBoostAppreciation) string {
	identity := strings.TrimSpace(item.Name)
	if item.Username != "" {
		identity = "@" + item.Username
	}
	if strings.HasPrefix(strings.ToLower(item.Locale), "zh") {
		if identity == "" {
			return telegramformat.Escape("感谢为群组助力！每次助力都让我们的社区更好。")
		}
		return telegramformat.Escape("感谢 " + identity + " 为群组助力！每次助力都让我们的社区更好。")
	}
	if identity == "" {
		return telegramformat.Escape("Thank you for boosting the group! Your support helps our community.")
	}
	return telegramformat.Escape("Thank you " + identity + " for boosting the group! Your support helps our community.")
}
