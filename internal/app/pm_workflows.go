package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegrampm"
)

func newApplicationPMWorkflows(store *database.Store, settings *admin.SettingsService, sender *queuedTelegram, admins []int64,
	dispatcher *providerops.Dispatcher, worker *outbox.Worker, users *admin.UserWorkflows, catalogService *catalog.Service,
	publicBaseURL string, timezone *time.Location, vault *secret.Vault) (*telegrampm.Service, error) {
	store.ConfigurePMContentVault(vault)
	zone := "UTC"
	if timezone != nil {
		zone = timezone.String()
	}
	profiles := pmProfileReader{users: users, catalog: catalogService, store: store}
	return newPMWorkflows(store, settings, sender, admins, dispatcher, worker, profiles, publicBaseURL, zone)
}

func newPMWorkflows(store *database.Store, settings *admin.SettingsService, sender *queuedTelegram, admins []int64,
	dispatcher *providerops.Dispatcher, worker *outbox.Worker, profiles telegrampm.ProfileFactsReader, publicBaseURL, timezone string) (*telegrampm.Service, error) {
	identities := make(map[int64]bool, len(admins))
	for _, id := range admins {
		identities[id] = true
	}
	relay := &telegrampm.Worker{Repository: store, Settings: settings, Sender: sender, AdminIDs: identities,
		Profiles: profiles, PublicBaseURL: publicBaseURL, Timezone: timezone}
	for _, kind := range []string{providerops.KindTelegramPMRelay, providerops.KindTelegramPMProfile, providerops.KindTelegramPMRepair} {
		if err := dispatcher.Register(kind, relay); err != nil {
			return nil, err
		}
	}
	if err := worker.Register("telegram_pm_notice", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		var item model.PMNotice
		if err := json.Unmarshal([]byte(job.Payload), &item); err != nil {
			return fmt.Errorf("decode PM guidance: %w", err)
		}
		if item.ChatID <= 0 || item.ReplyMessageID <= 0 {
			return fmt.Errorf("invalid PM guidance target")
		}
		return sender.SendMarkdownV2Message(ctx, item.ChatID, item.ReplyMessageID, telegramformat.Escape(telegrampm.NoticeText(item)))
	})); err != nil {
		return nil, err
	}
	return &telegrampm.Service{Repository: store, Settings: settings, Callbacks: sender, AdminIDs: identities}, nil
}
