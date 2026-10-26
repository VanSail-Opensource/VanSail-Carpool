package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func registerPricingChangeNotice(worker *outbox.Worker, sender *queuedTelegram, admins []int64) error {
	return worker.Register("pricing_change_notice", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		var in struct{ Actor, Command, Scope, Reason string }
		if err := json.Unmarshal([]byte(job.Payload), &in); err != nil {
			return err
		}
		text := telegramformat.Escape(fmt.Sprintf("Pricing change applied / 定价变更已生效\n%s · %s\n%s\n%s", in.Command, in.Scope, in.Actor, in.Reason))
		for _, admin := range admins {
			if err := sender.SendMarkdownV2Message(ctx, admin, 0, text); err != nil {
				return err
			}
		}
		return nil
	}))
}
