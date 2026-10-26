package app

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/entitlements"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func registerCoreOutboxHandlers(worker *outbox.Worker, store *database.Store, entitlementsWorker *entitlements.Worker, native *xnode.Service,
	embyService *emby.Service, paymentAnnouncements *billing.PaymentAnnouncementWorker,
	affiliateNotifications *affiliates.NotificationWorker, userNotifications *notifications.Worker,
	scanWorker *connections.Worker, expiryWorker *connections.ExpiryWorker, dispatcher *providerops.Dispatcher) error {
	if err := worker.Register(jobpayload.PaymentSuccessAnnouncementKind, paymentAnnouncements); err != nil {
		return fmt.Errorf("register payment announcement outbox handler: %w", err)
	}
	for _, kind := range []string{jobpayload.AffiliateSuccessKind, jobpayload.AffiliateTierUpgradeKind} {
		if err := worker.Register(kind, affiliateNotifications); err != nil {
			return fmt.Errorf("register %s outbox handler: %w", kind, err)
		}
	}
	if err := worker.Register(jobpayload.UserNotificationKind, userNotifications); err != nil {
		return fmt.Errorf("register user notification outbox handler: %w", err)
	}
	for _, kind := range []string{"xnode_apply_entitlement", "xnode_sync_user", jobpayload.ContinuityKind} {
		if err := worker.Register(kind, entitlementsWorker); err != nil {
			return fmt.Errorf("register %s outbox handler: %w", kind, err)
		}
	}
	if err := worker.Register("purchase_expire", outbox.HandlerFunc(purchaseExpiry(store, native))); err != nil {
		return err
	}
	if err := worker.Register(connections.ScanRequestOutboxKind, scanWorker); err != nil {
		return err
	}
	if err := worker.Register(connections.BlockExpiryOutboxKind, expiryWorker); err != nil {
		return err
	}
	if err := registerProviderDispatcher(worker, dispatcher); err != nil {
		return err
	}
	if err := worker.Register(emby.ProvisionOutboxKind, outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		accountID, err := jobpayload.TargetID(job, "accountId")
		if err != nil {
			return err
		}
		return embyService.HandleProvisionJob(ctx, accountID)
	})); err != nil {
		return err
	}
	return worker.Register("questionnaire_settlement", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		importID, err := jobpayload.TargetID(job, "importId")
		if err != nil {
			return err
		}
		_, err = store.SettleQuestionnaireImport(ctx, importID, time.Now().UTC())
		return err
	}))
}
