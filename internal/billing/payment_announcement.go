package billing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// PaymentAnnouncementChatIDSetting selects the standalone Telegram channel
// that receives successful provider-payment announcements.
const PaymentAnnouncementChatIDSetting = "telegram.payment_announcement_chat_id"

// PaymentAnnouncementSettings loads the optional standalone channel ID.
type PaymentAnnouncementSettings interface {
	Optional(context.Context, string) (string, error)
}

// PaymentAnnouncementSender is the narrow Telegram delivery contract.
type PaymentAnnouncementSender interface {
	SendMarkdownV2Message(context.Context, int64, int64, string) error
}

// PaymentAnnouncementWorker sends immutable settlement snapshots from the
// durable outbox.
type PaymentAnnouncementWorker struct {
	settings PaymentAnnouncementSettings
	sender   PaymentAnnouncementSender
}

// NewPaymentAnnouncementWorker creates the durable Telegram delivery handler.
func NewPaymentAnnouncementWorker(settings PaymentAnnouncementSettings, sender PaymentAnnouncementSender) *PaymentAnnouncementWorker {
	return &PaymentAnnouncementWorker{settings: settings, sender: sender}
}

// HandleOutbox sends one payment announcement or skips delivery when no
// standalone channel has been configured.
func (w *PaymentAnnouncementWorker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	payload, err := jobpayload.DecodePaymentSuccessAnnouncement(job)
	if err != nil {
		return err
	}
	if err := validatePaymentSuccessAnnouncement(payload); err != nil {
		return err
	}
	rawChatID, err := w.settings.Optional(ctx, PaymentAnnouncementChatIDSetting)
	if err != nil {
		return fmt.Errorf("load payment announcement channel: %w", err)
	}
	rawChatID = strings.TrimSpace(rawChatID)
	if rawChatID == "" {
		return nil
	}
	chatID, err := strconv.ParseInt(rawChatID, 10, 64)
	if err != nil || chatID == 0 {
		return errors.New("payment announcement channel must be a non-zero integer")
	}
	if err := w.sender.SendMarkdownV2Message(ctx, chatID, 0, formatPaymentSuccessAnnouncement(payload)); err != nil {
		return fmt.Errorf("send payment success announcement: %w", err)
	}
	return nil
}
