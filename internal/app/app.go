// Package app wires the single-container TX Carpool application.
package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/abuse"
	"github.com/txyyddss/Remna-User-Panel/internal/accounts"
	"github.com/txyyddss/Remna-User-Panel/internal/admin"
	"github.com/txyyddss/Remna-User-Panel/internal/affiliates"
	"github.com/txyyddss/Remna-User-Panel/internal/billing"
	"github.com/txyyddss/Remna-User-Panel/internal/catalog"
	"github.com/txyyddss/Remna-User-Panel/internal/compensation"
	"github.com/txyyddss/Remna-User-Panel/internal/connectivity"
	"github.com/txyyddss/Remna-User-Panel/internal/coupons"
	"github.com/txyyddss/Remna-User-Panel/internal/emby"
	"github.com/txyyddss/Remna-User-Panel/internal/entitlements"
	"github.com/txyyddss/Remna-User-Panel/internal/httpapi"
	"github.com/txyyddss/Remna-User-Panel/internal/integrations/telegram"
	"github.com/txyyddss/Remna-User-Panel/internal/integrations/turnstile"
	"github.com/txyyddss/Remna-User-Panel/internal/maintenance"
	"github.com/txyyddss/Remna-User-Panel/internal/notifications"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/backup"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/config"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/databaseadmin"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/outbox"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/secret"
	"github.com/txyyddss/Remna-User-Panel/internal/questionnaires"
	"github.com/txyyddss/Remna-User-Panel/internal/rollover"
	productstats "github.com/txyyddss/Remna-User-Panel/internal/statistics"
	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
	"github.com/txyyddss/Remna-User-Panel/internal/webui"
)

// New opens persistence, constructs integrations, and builds the HTTP router.
func New(cfg config.Config, logger *slog.Logger) (*Application, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	migrationVersions, err := database.MigrationVersions()
	if err != nil {
		return nil, err
	}
	if _, restoreErr := backup.ApplyPendingRestore(ctx, cfg.DatabasePath, migrationVersions); restoreErr != nil {
		if _, statErr := os.Stat(cfg.DatabasePath); statErr != nil {
			return nil, fmt.Errorf("apply pending database restore: %w", restoreErr)
		}
		logger.Error("pending database restore rolled back or was rejected", "error", restoreErr)
	}
	db, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	cleanup := func(err error) (*Application, error) {
		_ = db.Close()
		return nil, err
	}
	store := database.NewStore(db)
	store.SetLogger(logger)
	if _, err := backup.RecordStartupRestore(ctx, db, cfg.DatabasePath); err != nil {
		return cleanup(fmt.Errorf("record startup database restore: %w", err))
	}
	vault, err := secret.NewVault(cfg.MasterKey)
	if err != nil {
		return cleanup(err)
	}
	settings := admin.NewSettingsService(store, vault)
	settings.SetPaymentProfileRepository(store)
	paymentChannels := billing.NewPaymentChannelCache()
	settings.SetPaymentProfileChannels(paymentChannels)
	if err := ensureBootstrapSettings(ctx, store, vault); err != nil {
		return cleanup(err)
	}
	upstreams, err := newProviderQueues()
	if err != nil {
		return cleanup(err)
	}
	verifier, err := telegram.NewInitDataVerifier(cfg.TelegramBotToken, cfg.InitDataMaxAge)
	if err != nil {
		return cleanup(err)
	}
	telegramClient, err := telegram.NewClient(cfg.TelegramBotToken)
	if err != nil {
		return cleanup(err)
	}
	remna := newRemnaAdapter(settings, upstreams.remnawave)
	remna.squadPreferences = store
	connectivityService := connectivity.NewService(settings, connectivitySource{adapter: remna}, connectivity.NewXrayProbe(), store, upstreams.connectivity)
	settings.SetConnectivityValidation(connectivityService.ValidateConfig, connectivityService.Invalidate)
	queuedTelegramClient := &queuedTelegram{client: telegramClient, queue: upstreams.telegram}
	settings.SetPMForumValidator(validatePMForum(queuedTelegramClient))
	affiliateService := affiliates.NewService(store, queuedTelegramClient)
	telegramBridge := telegramAdapter{client: queuedTelegramClient}
	usdtService := usdt.NewService(store, settings)
	usdtService.OnError = func(code string) { logger.Error("USDT collector interrupted", "code", code) }
	paymentBridge := paymentAdapter{settings: settings, telegram: queuedTelegramClient, queue: upstreams.payment, users: store, usdt: usdtService}
	paymentProfiles := newPaymentProfileManager(settings, paymentChannels, upstreams.payment, queuedTelegramClient, store, cfg.AdminTelegramIDs, cfg.PublicBaseURL, logger)
	backupService := backup.NewService(db, store, filepath.Join(cfg.DataDir, "backups"), cfg.BackupRetention)
	reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	reconcileErr := backupService.ReconcileUploads(reconcileCtx, migrationVersions)
	reconcileCancel()
	if reconcileErr != nil {
		return cleanup(fmt.Errorf("reconcile interrupted backup uploads: %w", reconcileErr))
	}
	maintenanceService := maintenance.NewService(store, backupService, cfg.Timezone)
	databaseEditor := databaseadmin.NewService(db, backupService, vault, logger)
	databaseAdminHTTP := httpapi.NewDatabaseAdministrationHTTP(databaseEditor, backupService, migrationVersions)
	databaseAdminHTTP.SetBackupUploadMaxBytes(cfg.BackupUploadMaxBytes)
	accountsService := accounts.NewService(store, store, initDataAdapter{verifier: verifier}, telegramBridge, remna, settings, cfg.AdminTelegramIDs, cfg.SessionTTL)
	catalogService := catalog.NewService(store, remna, 2*time.Minute)
	catalogService.SetConnectivity(connectivityService)
	catalogService.SetAutomaticRenewalSettings(settings)
	billingService := billing.NewService(store, settings, paymentBridge, cfg.PublicBaseURL)
	billingService.SetBuiltinUSDT(usdtService)
	activityService := newActivityService(store, settings, queuedTelegramClient)
	couponService := coupons.NewService(store, nil)
	questionnaireService := questionnaires.NewService(store, questionnaires.CryptoCodeGenerator{}, nil)
	embyPrice := embySetupPrice(settings)
	embySecrets := emby.NewSecretBox(vault)
	embyService := emby.NewService(store, newEmbyAdapter(settings, upstreams.emby), embyPrice, embySecrets)
	embyOperations, err := emby.NewOperationService(embyService, store, embySecrets, cfg.MasterKey)
	if err != nil {
		return cleanup(err)
	}
	adminService := admin.NewService(store, settings, remna, backupService, paymentBridge)
	adminUserWorkflows := admin.NewUserWorkflows(store, remna)
	entitlementWorker := entitlements.NewWorker(store, remna, remna)
	rolloverWorker := rollover.NewService(store, remna)
	outboxWorker := outbox.NewWorker(store)
	paymentAnnouncementWorker := billing.NewPaymentAnnouncementWorker(settings, queuedTelegramClient)
	affiliateNotificationWorker := affiliates.NewNotificationWorker(queuedTelegramClient, store)
	userNotificationWorker := notifications.NewWorker(queuedTelegramClient, logger, cfg.Timezone, store)
	userNotificationScanner := notifications.NewScanner(store, remna, logger)
	memberServices, scanWorker, blockExpiryWorker, operationDispatcher, err := newMemberWorkflows(store, remna, vault, cfg.MasterKey)
	if err != nil {
		return cleanup(err)
	}
	ipLookupService, ipLookupWorker, ipLookupLive, err := newIPLookupService(store, settings, upstreams, operationDispatcher, cfg.MasterKey)
	if err != nil {
		return cleanup(err)
	}
	if err := registerIPLookupOutboxHandlers(outboxWorker, ipLookupWorker, ipLookupLive); err != nil {
		return cleanup(err)
	}
	ipLookupWorker.OnBackgroundError = func(code string) { logger.Error("background IP lookup interrupted", "code", code) }
	if err := registerPaymentOperationHandlers(operationDispatcher, billingService); err != nil {
		return cleanup(err)
	}
	if err := registerComboControlHandlers(operationDispatcher, store, entitlementWorker, catalogService.InvalidateDashboard); err != nil {
		return cleanup(err)
	}
	if err := registerMutationOperationHandlers(operationDispatcher, catalogService, embyOperations, questionnaireService, adminService); err != nil {
		return cleanup(err)
	}
	if err := registerMaintenanceOperationHandler(operationDispatcher, store, maintenanceService, cfg.DatabasePath, logger); err != nil {
		return cleanup(err)
	}
	if err := registerAdminOperationHandlers(operationDispatcher, store, remna); err != nil {
		return cleanup(err)
	}
	statisticsService := productstats.NewService(store, remna)
	compensationService := compensation.NewService(store, remna)
	abuseService := abuse.NewService(store, remna)
	if err := registerStatisticsOperationHandler(operationDispatcher, store, remna); err != nil {
		return cleanup(err)
	}
	if err := registerCoreOutboxHandlers(outboxWorker, store, entitlementWorker, rolloverWorker, embyService,
		paymentAnnouncementWorker, affiliateNotificationWorker, userNotificationWorker, scanWorker, blockExpiryWorker, operationDispatcher); err != nil {
		return cleanup(err)
	}
	if err := registerLuckyDrawOutboxHandlers(outboxWorker, store, activityService, queuedTelegramClient, settings, logger); err != nil {
		return cleanup(err)
	}
	if err := registerAdminUserOutboxHandlers(outboxWorker, store); err != nil {
		return cleanup(err)
	}
	if err := registerBoostAppreciationHandler(outboxWorker, settings, queuedTelegramClient); err != nil {
		return cleanup(err)
	}
	if err := registerAbuseOutboxHandlers(outboxWorker, store, remna, queuedTelegramClient); err != nil {
		return cleanup(err)
	}
	pmService, err := newApplicationPMWorkflows(store, settings, queuedTelegramClient, cfg.AdminTelegramIDs,
		operationDispatcher, outboxWorker, adminUserWorkflows, catalogService, affiliateService.BotUsername, cfg.Timezone, vault)
	if err != nil {
		return cleanup(err)
	}
	static, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		return cleanup(fmt.Errorf("open embedded frontend: %w", err))
	}
	static, err = webui.Preload(static)
	if err != nil {
		return cleanup(fmt.Errorf("preload embedded frontend: %w", err))
	}
	api, err := httpapi.New(httpapi.Dependencies{
		USDT:         usdtService,
		IPLookup:     ipLookupService,
		PM:           pmService,
		Connectivity: connectivityService,
		FirstEntry:   &accounts.FirstEntry{Repository: store, Settings: settings, Verifier: turnstile.New(upstreams.turnstile), Hostname: cfg.PublicBaseURL.Hostname(), AdminTelegramIDs: cfg.AdminTelegramIDs},
		Accounts:     accountsService, Catalog: catalogService, Connections: memberServices.connections,
		ConnectionDrops: memberServices.drops, PurchaseOperations: memberServices.purchases, Statistics: statisticsService,
		Billing: billingService, Activity: activityService,
		Coupons: couponService, Questionnaires: questionnaireService, Emby: embyService,
		EmbyOperations: embyOperations, EmbyPrice: embyPrice,
		Admin: adminService, AdminUsers: adminUserWorkflows, Compensation: compensationService, Settings: settings, DatabaseAdmin: databaseAdminHTTP,
		PaymentProfiles: paymentProfiles,
		Abuse:           abuseService, AbuseVault: vault,
		Store: store, Affiliates: affiliateService, Telegram: queuedTelegramClient, Webhooks: paymentBridge, PublicURL: cfg.PublicBaseURL, Static: static,
		Logger: logger, SessionTTL: cfg.SessionTTL, SecureCookies: cfg.PublicBaseURL.Scheme == "https",
		AdminTelegramIDs: cfg.AdminTelegramIDs, RequestSigningKey: cfg.MasterKey,
	})
	if err != nil {
		return cleanup(err)
	}
	httpServer := &http.Server{
		Addr: ":" + cfg.Port, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	return &Application{
		config: cfg, logger: logger, httpServer: httpServer, store: store, outbox: outboxWorker,
		backups: backupService, maintenance: maintenanceService, telegram: queuedTelegramClient, settings: settings,
		catalog: catalogService, billing: billingService, statistics: statisticsService, compensation: compensationService, affiliates: affiliateService, abuse: abuseService,
		notifications: userNotificationScanner, upstreams: upstreams, paymentProfiles: paymentProfiles,
		connectivity: connectivityService,
		ipLookup:     ipLookupWorker,
		usdt:         usdtService,
	}, nil
}
