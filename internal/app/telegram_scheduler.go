package app

import (
	"context"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func (a *Application) configureTelegram(ctx context.Context) {
	a.verifyAccountCleanupBot(ctx)
	if err := a.affiliates.RefreshBotIdentity(ctx); err != nil {
		a.logger.Warn("refresh Telegram bot identity", "error", err)
	}
	secret, err := a.settings.Plaintext(ctx, "telegram.webhook_secret")
	if err != nil {
		a.logger.Error("load Telegram webhook secret", "error", err)
		return
	}
	setupCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	webhookURL := *a.config.PublicBaseURL
	webhookURL.Path = strings.TrimRight(webhookURL.Path, "/") + "/api/v1/webhooks/telegram"
	if err := a.telegram.SetWebhook(setupCtx, telegram.WebhookConfig{
		URL: webhookURL.String(), SecretToken: secret,
		AllowedUpdates: telegram.DefaultAllowedUpdates(), MaxConnections: 20,
	}); err != nil {
		a.logger.Error("configure Telegram webhook", "error", err)
		return
	}
	if err := a.telegram.SetChatMenuButton(setupCtx, "Open VanSail Carpool", a.config.PublicBaseURL.String()); err != nil {
		a.logger.Error("configure Telegram menu", "error", err)
	}
	if err := a.configureTelegramCommands(setupCtx); err != nil {
		a.logger.Error("configure Telegram commands", "error", err)
	}
}
