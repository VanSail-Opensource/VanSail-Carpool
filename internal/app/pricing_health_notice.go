package app

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func registerPricingHealthNotice(worker *outbox.Worker, store *database.Store, sender *queuedTelegram, admins []int64) error {
	return worker.Register("pricing_health_notice", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		id, err := jobpayload.TargetID(job, "incidentId")
		if err != nil {
			return err
		}
		node, state, reason, err := store.PricingIncidentNotice(ctx, id)
		if err != nil {
			return err
		}
		message := telegramformat.Escape(fmt.Sprintf("Pricing health incident / 定价健康事件\n%s\n%s\n%s\nDynamic publication and new admissions require healthy telemetry. Existing accounting and paid entitlements are preserved. / 动态发布及新增准入需要健康遥测，现有计费和已购权益保持有效。", node, state, reason))
		for _, admin := range admins {
			if err = sender.SendMarkdownV2Message(ctx, admin, 0, message); err != nil {
				return err
			}
		}
		return nil
	}))
}
