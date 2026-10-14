package affiliates

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

type NotificationSender interface {
	SendMarkdownV2Message(context.Context, int64, int64, string) error
}

type notificationPreferences interface {
	TelegramNotificationAllowed(context.Context, int64, string) (bool, error)
}

type NotificationWorker struct {
	sender      NotificationSender
	preferences notificationPreferences
}

func NewNotificationWorker(sender NotificationSender, readers ...notificationPreferences) *NotificationWorker {
	w := &NotificationWorker{sender: sender}
	if len(readers) > 0 {
		w.preferences = readers[0]
	}
	return w
}

func (w *NotificationWorker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	switch job.Kind {
	case jobpayload.AffiliateSuccessKind:
		payload, err := jobpayload.DecodeAffiliateSuccess(job)
		if err != nil {
			return err
		}
		return w.deliver(ctx, payload.ChatID, job.Kind, formatSuccess(payload))
	case jobpayload.AffiliateTierUpgradeKind:
		payload, err := jobpayload.DecodeAffiliateTierUpgrade(job)
		if err != nil {
			return err
		}
		return w.deliver(ctx, payload.ChatID, job.Kind, formatUpgrade(payload))
	default:
		return fmt.Errorf("unsupported affiliate notification kind: %s", job.Kind)
	}
}

func (w *NotificationWorker) deliver(ctx context.Context, chatID int64, kind, body string) error {
	if w.preferences != nil {
		allowed, err := w.preferences.TelegramNotificationAllowed(ctx, chatID, kind)
		if err != nil {
			return fmt.Errorf("load referral notification preferences: %w", err)
		}
		if !allowed {
			return nil
		}
	}
	return w.sender.SendMarkdownV2Message(ctx, chatID, 0, telegramformat.Limit(body))
}

func formatSuccess(payload jobpayload.AffiliateSuccess) string {
	inviteeName := payload.InviteeName
	if strings.TrimSpace(inviteeName) == "" {
		inviteeName = "TX member"
		if payload.Locale == LocaleChinese {
			inviteeName = "TX 用户"
		}
	}
	if payload.Locale == LocaleChinese {
		return "🎉 *好友首充成功*\n\n好友：" + md(inviteeName) + "\n时间：" + md(payload.SettledAt) +
			"\n返佣：*" + md(formatTXB(payload.CommissionMinor)) + " TXB*\n结算等级：" + md(payload.TierName)
	}
	return "🎉 *Referral successful*\n\nMember: " + md(inviteeName) + "\nTime: " + md(payload.SettledAt) +
		"\nCommission: *" + md(formatTXB(payload.CommissionMinor)) + " TXB*\nSettlement tier: " + md(payload.TierName)
}

func formatUpgrade(payload jobpayload.AffiliateTierUpgrade) string {
	if payload.Locale == LocaleChinese {
		return "🚀 *等级提升*\n\n恭喜你升级至 *" + md(payload.TierName) + "*\n升级奖励：" + md(payload.RewardDescription)
	}
	return "🚀 *Tier upgraded*\n\nYou reached *" + md(payload.TierName) + "*\nUpgrade reward: " + md(payload.RewardDescription)
}

func FormatReferralWelcome(locale, inviter string) string {
	if strings.TrimSpace(inviter) == "" {
		inviter = "TX member"
		if locale == LocaleChinese {
			inviter = "TX 用户"
		}
	}
	if locale == LocaleChinese {
		return "👋 你由 *" + md(inviter) + "* 邀请加入 VanSail Carpool。"
	}
	return "👋 You were invited to VanSail Carpool by *" + md(inviter) + "*\\."
}

func formatTXB(minor int64) string {
	return strconv.FormatInt(minor/100, 10) + "." + fmt.Sprintf("%02d", minor%100)
}

func md(value string) string {
	return telegramformat.Escape(value)
}
