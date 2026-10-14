package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type drawGroupSetting interface {
	Optional(context.Context, string) (string, error)
}
type luckyDrawOutbox struct {
	store    *database.Store
	activity *activity.Service
	telegram *queuedTelegram
	settings drawGroupSetting
	logger   *slog.Logger
}

func registerLuckyDrawOutboxHandlers(worker *outbox.Worker, store *database.Store, activityService *activity.Service,
	sender *queuedTelegram, settings drawGroupSetting, logger *slog.Logger) error {
	handler := &luckyDrawOutbox{store: store, activity: activityService, telegram: sender, settings: settings, logger: logger}
	for _, kind := range []string{"draw_telegram_publish", "draw_telegram_update", "draw_telegram_delete",
		"draw_raffle_reply", "draw_raffle_settle", "draw_raffle_complete", "draw_raffle_private", "draw_instant_announcement",
		"draw_raffle_price_notice"} {
		if err := worker.Register(kind, handler); err != nil {
			return err
		}
	}
	return nil
}
func (w *luckyDrawOutbox) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	switch job.Kind {
	case "draw_telegram_publish", "draw_telegram_update", "draw_telegram_delete", "draw_raffle_settle", "draw_raffle_complete":
		id, err := jobpayload.TargetID(job, "drawId")
		if err != nil {
			return err
		}
		switch job.Kind {
		case "draw_telegram_publish":
			return w.publish(ctx, id)
		case "draw_telegram_update":
			return w.update(ctx, id)
		case "draw_telegram_delete":
			return w.delete(ctx, id)
		case "draw_raffle_settle":
			_, err = w.activity.SettleRaffle(ctx, id)
			return err
		default:
			return w.complete(ctx, id)
		}
	case "draw_raffle_reply":
		id, err := jobpayload.TargetID(job, "ticketId")
		if err != nil {
			return err
		}
		return w.reply(ctx, id)
	case "draw_raffle_price_notice":
		return w.priceNotice(ctx, job)
	case "draw_raffle_private", "draw_instant_announcement":
		id, err := jobpayload.TargetID(job, "resultId")
		if err != nil {
			return err
		}
		return w.resultNotice(ctx, id, job.Kind)
	default:
		return fmt.Errorf("unsupported draw job %q", job.Kind)
	}
}
func (w *luckyDrawOutbox) priceNotice(ctx context.Context, job model.OutboxJob) error {
	var payload struct {
		DrawID           string `json:"drawId"`
		Revision         int    `json:"revision"`
		PreviousFeeMinor int64  `json:"previousFeeMinor"`
		NewFeeMinor      int64  `json:"newFeeMinor"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return err
	}
	if payload.DrawID == "" || payload.Revision <= 0 {
		return fmt.Errorf("invalid raffle price notice")
	}
	draw, err := w.store.LuckyDrawByID(ctx, payload.DrawID)
	if err != nil {
		return err
	}
	if draw.Status != "open" && draw.Status != "settling" {
		return nil
	}
	key := "raffle:price:" + payload.DrawID + ":" + strconv.Itoa(payload.Revision)
	if _, done, readErr := w.store.DrawDelivery(ctx, key); readErr != nil || done {
		return readErr
	}
	body := "💰 *Entry price updated*\n" + drawEscape(draw.Name) + "\n" +
		drawEscape(model.TXBMoney(payload.PreviousFeeMinor).Display) + " → " +
		drawEscape(model.TXBMoney(payload.NewFeeMinor).Display) + "\n" +
		"Existing seats keep their original charge"
	messageID, err := w.telegram.PublishMarkdownV2Message(ctx, draw.GroupChatID, body)
	if err != nil {
		return err
	}
	return w.store.RecordDrawDelivery(ctx, key, draw.GroupChatID, messageID, time.Now().UTC())
}
func (w *luckyDrawOutbox) publish(ctx context.Context, id string) error {
	draw, err := w.store.LuckyDrawByID(ctx, id)
	if err != nil {
		return err
	}
	if draw.Status != "publishing" {
		return nil
	}
	key := "raffle:publish:" + id
	if messageID, recorded, readErr := w.store.DrawDelivery(ctx, key); readErr != nil {
		return readErr
	} else if recorded {
		return w.store.ConfirmRafflePublished(ctx, id, messageID, time.Now().UTC())
	}
	body := drawAnnouncement(draw, 0)
	messageID, err := w.telegram.PublishMarkdownV2Message(ctx, draw.GroupChatID, body)
	if err != nil {
		var apiErr *telegram.APIError
		if errors.As(err, &apiErr) {
			return err
		}
		w.logger.Error("raffle publication requires review", "draw_id", id, "error", err)
		return nil // Unknown delivery must never open sales or auto-send a duplicate.
	}
	if err = w.store.RecordDrawDelivery(ctx, key, draw.GroupChatID, messageID, time.Now().UTC()); err != nil {
		w.logger.Error("raffle message sent but ID could not be stored", "draw_id", id, "message_id", messageID, "error", err)
		return nil
	}
	if err = w.store.ConfirmRafflePublished(ctx, id, messageID, time.Now().UTC()); err != nil {
		if errors.Is(err, database.ErrConflict) {
			return nil
		}
		return err
	}
	return nil
}
func (w *luckyDrawOutbox) update(ctx context.Context, id string) error {
	draw, err := w.store.LuckyDrawByID(ctx, id)
	if err != nil {
		return err
	}
	if draw.Status != "open" && draw.Status != "settling" {
		return nil
	}
	if draw.AnnouncementMessageID <= 0 {
		return nil
	}
	seats, err := w.store.RaffleProgress(ctx, id)
	if err != nil {
		return err
	}
	body := drawAnnouncement(draw, seats)
	err = w.telegram.EditMarkdownV2Message(ctx, draw.GroupChatID, draw.AnnouncementMessageID, body)
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.Description), "message is not modified") {
		return nil
	}
	return err
}
func (w *luckyDrawOutbox) delete(ctx context.Context, id string) error {
	draw, err := w.store.LuckyDrawByID(ctx, id)
	if err != nil {
		return err
	}
	messageID := draw.AnnouncementMessageID
	if messageID <= 0 {
		var recorded bool
		messageID, recorded, err = w.store.DrawDelivery(ctx, "raffle:publish:"+id)
		if err != nil {
			return err
		}
		if !recorded {
			return nil
		}
	}
	if err = w.telegram.DeleteMessage(ctx, draw.GroupChatID, messageID); err == nil {
		return nil
	}
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.Description), "message to delete not found") {
		return nil
	}
	body := "🎟 *" + drawEscape(draw.Name) + "*\n🔒 This raffle is closed"
	editErr := w.telegram.EditMarkdownV2Message(ctx, draw.GroupChatID, messageID, body)
	if errors.As(editErr, &apiErr) && strings.Contains(strings.ToLower(apiErr.Description), "message to edit not found") {
		return nil
	}
	return editErr
}
func (w *luckyDrawOutbox) reply(ctx context.Context, id string) error {
	ticket, err := w.store.RaffleTicketByID(ctx, id)
	if err != nil {
		return err
	}
	if ticket.Status == "refunded" {
		return nil
	}
	key := "raffle:reply:" + id
	if _, done, err := w.store.DrawDelivery(ctx, key); err != nil || done {
		return err
	}
	receipt := ticket.Receipt
	body := "🎟 Seat confirmed\n" + drawProgress(receipt.Seats, receipt.Threshold) + "\n" +
		drawEscape(fmt.Sprintf("%d / %d seats", receipt.Seats, receipt.Threshold)) + "\n" +
		drawEscape(fmt.Sprintf("Your seats: %d", receipt.UserSeats))
	messageID, err := w.telegram.ReplyMarkdownV2Message(ctx, ticket.ChatID, ticket.MessageID, body)
	if err != nil {
		return err
	}
	return w.store.RecordDrawDelivery(ctx, key, ticket.ChatID, messageID, time.Now().UTC())
}
func (w *luckyDrawOutbox) groupID(ctx context.Context) (int64, error) {
	raw, err := w.settings.Optional(ctx, "telegram.group_chat_id")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
}
