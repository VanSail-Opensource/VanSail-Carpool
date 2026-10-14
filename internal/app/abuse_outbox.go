package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func registerAbuseOutboxHandlers(worker *outbox.Worker, store *database.Store, remna remnaAdapter, telegram *queuedTelegram) error {
	if err := worker.Register("abuse_punishment", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		return handleAbusePunishment(ctx, job, store, remna)
	})); err != nil {
		return err
	}
	if err := worker.Register("abuse_restore", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		return handleAbuseRestore(ctx, job, store, remna)
	})); err != nil {
		return err
	}
	return worker.Register(jobpayload.UserAbuseNotificationKind, outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		return handleAbuseNotification(ctx, job, store, telegram)
	}))
}
func handleAbusePunishment(ctx context.Context, job model.OutboxJob, store *database.Store, remna remnaAdapter) error {
	id, err := jobpayload.TargetID(job, "recordId")
	if err != nil {
		return err
	}
	item, err := store.AbuseJob(ctx, id)
	if err != nil {
		return err
	}
	var punishmentErr error
	switch string(item.Action) {
	case "warning", "none":
	case "subscription_revoke":
		punishmentErr = remna.AbuseRevoke(ctx, item.RemoteUserID)
	case "temporary_ban":
		punishmentErr = remna.AbuseSetStatus(ctx, item.RemoteUserID, remnawave.UserStatusDisabled)
	case "ip_ban":
		punishmentErr = handleAbuseIPBan(ctx, id, item, store, remna)
	default:
		return fmt.Errorf("unsupported abuse action %q", item.Action)
	}
	if punishmentErr != nil {
		return punishmentErr
	}
	return store.MarkPunishmentCompleted(ctx, id, time.Now().UTC())
}
func handleAbuseRestore(ctx context.Context, job model.OutboxJob, store *database.Store, remna remnaAdapter) error {
	userID, err := jobpayload.TargetID(job, "userId")
	if err != nil {
		return err
	}
	remoteID, err := store.AbuseRestoreRemoteID(ctx, userID)
	if err == database.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if err = remna.AbuseSetStatus(ctx, remoteID, remnawave.UserStatusActive); err != nil {
		return err
	}
	return store.CompleteAbuseRestore(ctx, userID, time.Now().UTC())
}
func handleAbuseNotification(ctx context.Context, job model.OutboxJob, store *database.Store, telegram *queuedTelegram) error {
	recordID, err := jobpayload.TargetID(job, "recordId")
	if err != nil {
		return err
	}
	rawID, err := jobpayload.TargetID(job, "telegramId")
	if err != nil {
		return err
	}
	var telegramID int64
	if _, err = fmt.Sscan(rawID, &telegramID); err != nil {
		return err
	}
	delivery, err := store.AbuseDelivery(ctx, recordID, telegramID)
	if err != nil {
		return err
	}
	if delivery.Delivered {
		return nil
	}
	user, err := store.UserByID(ctx, delivery.UserID)
	if err != nil {
		return err
	}
	if user.TelegramID == telegramID {
		allowed, err := store.TelegramNotificationAllowed(ctx, telegramID, jobpayload.UserAbuseNotificationKind)
		if err != nil {
			return err
		}
		if !allowed {
			return nil
		}
	}
	message := abuseMessage(delivery)
	if err = telegram.SendMarkdownV2Message(ctx, telegramID, 0, message); err != nil {
		return err
	}
	return store.MarkAbuseDelivery(ctx, recordID, telegramID, time.Now().UTC())
}
func abuseMessage(delivery database.AbuseDelivery) string {
	username := strings.TrimSpace(delivery.Username)
	chinese := delivery.Locale == "zh-CN"
	if username == "" {
		username = "Unknown"
		if chinese {
			username = "未知"
		}
	}
	const timeFormat = "2006-01-02 15:04 UTC"
	label := [5]string{"User: ", "Time: ", "Reason: ", "Action: ", "Until: "}
	title, warning := "⚠️ *Abuse detected*", "Stop abusive activity now. Further violations may lead to a ban without a refund."
	if chinese {
		label = [5]string{"用户: ", "时间: ", "原因: ", "处罚: ", "到期时间: "}
		title, warning = "⚠️ *滥用检测*", "请立即停止您的滥用行为，否则可能被封禁且不予退款。"
	}
	parts := []string{title,
		label[0] + telegramformat.Escape(username),
		label[1] + telegramformat.Escape(delivery.OccurredAt.UTC().Format(timeFormat)),
		label[2] + telegramformat.Escape(fmt.Sprintf("%s (QPS %d/%d)", delivery.Reason, delivery.QPS, delivery.Limit)),
		label[3] + telegramformat.Escape(abuseActionLabel(delivery.Action, chinese)),
	}
	if delivery.ExpiresAt != nil {
		parts = append(parts, label[4]+telegramformat.Escape(delivery.ExpiresAt.UTC().Format(timeFormat)))
	}
	parts = append(parts, telegramformat.Escape(warning))
	return telegramformat.Limit(strings.Join(parts, "\n"))
}
func abuseActionLabel(action abuse.Action, chinese bool) string {
	if chinese {
		switch action {
		case abuse.ActionWarning:
			return "警告"
		case abuse.ActionIPBan:
			return "封禁 IP"
		case abuse.ActionRevoke:
			return "撤销订阅"
		case abuse.ActionTemporaryBan:
			return "临时封禁"
		}
	} else {
		switch action {
		case abuse.ActionWarning:
			return "Warning"
		case abuse.ActionIPBan:
			return "IP ban"
		case abuse.ActionRevoke:
			return "Subscription revoked"
		case abuse.ActionTemporaryBan:
			return "Temporary ban"
		}
	}
	return string(action)
}
