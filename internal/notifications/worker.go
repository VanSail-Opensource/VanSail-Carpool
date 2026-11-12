package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
)

// Sender delivers one MarkdownV2 message through the queued Telegram adapter.
type Sender interface {
	SendMarkdownV2Message(context.Context, int64, int64, string) error
}

// Worker owns durable private-chat delivery.
type Worker struct {
	sender      Sender
	logger      *slog.Logger
	location    *time.Location
	preferences preferences.Reader
}

// NewWorker creates a user-notification outbox handler.
func NewWorker(sender Sender, logger *slog.Logger, location *time.Location, readers ...preferences.Reader) *Worker {
	w := &Worker{sender: sender, logger: logger, location: location}
	if len(readers) > 0 {
		w.preferences = readers[0]
	}
	return w
}

// HandleOutbox validates, formats, and sends one immutable event.
func (w *Worker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	// Acknowledge pre-retirement notification payloads without restoring their
	// formatter, producer or product controls. Their audit facts remain stored.
	var retired struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal([]byte(job.Payload), &retired) == nil && (retired.Kind == "node_compensation" || retired.Kind == "automatic_traffic_reset_insufficient") {
		return nil
	}
	payload, err := jobpayload.DecodeUserNotification(job)
	if err != nil {
		return err
	}
	if w.preferences != nil {
		p, err := w.preferences.UserPreferences(ctx, payload.UserID)
		if err != nil {
			return fmt.Errorf("load notification preferences: %w", err)
		}
		allowed, err := p.Allows(payload.Kind)
		if err != nil {
			return err
		}
		if !allowed {
			if w.logger != nil {
				w.logger.Info("user notification suppressed", "event_key", payload.EventKey, "kind", payload.Kind, "user_id", payload.UserID)
			}
			return nil
		}
	}
	message, err := Format(payload, w.location)
	if err != nil {
		return fmt.Errorf("format user notification %s: %w", payload.EventKey, err)
	}
	if err := w.sender.SendMarkdownV2Message(ctx, payload.ChatID, 0, message); err != nil {
		var api *telegram.APIError
		if errors.As(err, &api) && api != nil {
			description := strings.ToLower(strings.TrimSpace(api.Description))
			unreachable := api.ErrorCode == 403 && (description == "forbidden: bot was blocked by the user" || description == "forbidden: user is deactivated") || api.ErrorCode == 400 && description == "bad request: chat not found"
			if unreachable && api.HTTPStatus == api.ErrorCode && api.RetryAfter == 0 {
				return &outbox.TerminalError{Reason: "telegram_recipient_unreachable", Cause: err}
			}
			if api.ErrorCode == 429 && api.RetryAfter > 0 {
				return &outbox.WaitError{Reason: "telegram_rate_limited", Cause: err, After: time.Duration(min(api.RetryAfter, 86400)) * time.Second}
			}
		}
		if w.logger != nil {
			w.logger.Warn("user notification delivery will retry", "event_key", payload.EventKey, "kind", payload.Kind,
				"user_id", payload.UserID, "attempt", job.Attempts, "error", err)
		}
		return fmt.Errorf("send user notification %s: %w", payload.EventKey, err)
	}
	if w.logger != nil {
		w.logger.Info("user notification delivered", "event_key", payload.EventKey, "kind", payload.Kind, "user_id", payload.UserID)
	}
	return nil
}
