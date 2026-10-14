package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
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
