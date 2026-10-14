package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (a *Application) configureTelegram(ctx context.Context) {
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

func (a *Application) reconcileStars(ctx context.Context) {
	transactions, err := a.telegram.GetStarTransactions(ctx, 0, 100)
	if err != nil {
		a.logger.Error("Stars reconciliation failed", "error", err)
		return
	}
	for _, transaction := range transactions {
		event, refund, ok := normalizeStarTransaction(transaction)
		if !ok {
			continue
		}
		if !refund {
			if _, _, err := a.billing.Settle(ctx, event); err != nil && !errors.Is(err, database.ErrConflict) && !errors.Is(err, database.ErrNotFound) {
				a.logger.Error("reconcile Stars credit", "transaction_id", transaction.ID, "error", err)
			}
			continue
		}
		if _, err := a.billing.ValidateEvent(ctx, event); err == nil {
			if _, err := a.store.RefundPayment(ctx, nil, event.OrderID, "Telegram Stars reconciliation refund", time.Now().UTC()); err != nil && !errors.Is(err, database.ErrConflict) {
				a.logger.Error("reconcile Stars refund", "transaction_id", transaction.ID, "error", err)
			} else if err := a.store.ResolvePaymentRefundOperation(ctx, event.OrderID, transaction.ID, time.Now().UTC()); err != nil {
				a.logger.Error("resolve Stars refund operation", "transaction_id", transaction.ID, "error", err)
			}
		} else if !errors.Is(err, database.ErrConflict) && !errors.Is(err, database.ErrNotFound) {
			a.logger.Error("reconcile Stars refund", "transaction_id", transaction.ID, "error", err)
		}
	}
}

func normalizeStarTransaction(transaction telegram.StarTransaction) (billing.ProviderEvent, bool, bool) {
	if transaction.NanostarAmount != 0 || transaction.ID == "" {
		return billing.ProviderEvent{}, false, false
	}
	amount := transaction.Amount
	if amount < 0 {
		amount = -amount
	}
	if amount == 0 {
		return billing.ProviderEvent{}, false, false
	}
	partner := transaction.Source
	refund := false
	if partner == nil {
		partner = transaction.Receiver
		refund = partner != nil
	}
	if partner == nil || partner.Type != "user" || partner.TransactionType != "invoice_payment" || partner.InvoicePayload == "" || partner.User.ID <= 0 {
		return billing.ProviderEvent{}, false, false
	}
	telegramID := partner.User.ID
	event := billing.ProviderEvent{
		Provider: "stars", OrderID: partner.InvoicePayload, TradeID: transaction.ID,
		PayableAmount: strconv.FormatInt(amount, 10), PayableCurrency: "XTR", TelegramID: &telegramID,
	}
	if !refund {
		event.DedupeKey = transaction.ID
	}
	return event, refund, true
}
