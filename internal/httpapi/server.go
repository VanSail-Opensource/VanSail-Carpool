// Package httpapi exposes the versioned JSON API and embedded Vue application.
package httpapi

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/browserauth"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegrampm"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const sessionCookie = "txc_session"

// PaymentWebhookVerifier authenticates legacy provider callbacks before domain settlement.
type PaymentWebhookVerifier interface {
	VerifyEZPay(context.Context, url.Values) (billing.ProviderEvent, bool, error)
	VerifyBEPusdt(context.Context, []byte) (billing.ProviderEvent, int, error)
}

type bepusdtUnsignedVerifier interface {
	VerifyBEPusdtUnsigned(context.Context, []byte) (billing.ProviderEvent, int, error)
}

type telegramProvider interface {
	SendMarkdownV2Message(context.Context, int64, int64, string) error
	AnswerPreCheckoutQuery(context.Context, string, bool, string) error
}

type Dependencies struct {
	APITokens          *apitoken.Service
	XNode              *xnode.Service
	XNodeMaster        *xnode.Master
	BrowserAuth        *browserauth.Service
	USDT               *usdt.Service
	IPLookup           *iplookup.Service
	Connectivity       *connectivity.Service
	PM                 *telegrampm.Service
	FirstEntry         *accounts.FirstEntry
	Accounts           *accounts.Service
	Catalog            *catalog.Service
	Connections        *connections.Service
	ConnectionDrops    *connections.DropService
	PurchaseOperations *purchaseops.Service
	Statistics         *productstats.Service
	Billing            *billing.Service
	Activity           *activity.Service
	Coupons            *coupons.Service
	Questionnaires     *questionnaires.Service
	Emby               *emby.Service
	EmbyOperations     *emby.OperationService
	EmbyPrice          emby.PriceSource
	Admin              *admin.Service
	AdminUsers         *admin.UserWorkflows
	Compensation       *compensation.Service
	Settings           *admin.SettingsService
	PaymentProfiles    paymentProfileLifecycle
	DatabaseAdmin      *DatabaseAdministrationHTTP
	Store              *database.Store
	Affiliates         *affiliates.Service
	Telegram           telegramProvider
	Webhooks           PaymentWebhookVerifier
	PublicURL          *url.URL
	Static             fs.FS
	Logger             *slog.Logger
	RequestSigningKey  []byte
	SessionTTL         time.Duration
	SecureCookies      bool
	AdminTelegramIDs   []int64
}

// Server owns HTTP routing and transport-only validation.
type Server struct {
	deps             Dependencies
	requests         *requestauth.Verifier
	authLimiter      *authLimiter
	router           http.Handler
	adminTelegramIDs map[int64]struct{}
}

// New constructs all public, authenticated, admin, and webhook routes.
func New(deps Dependencies) (*Server, error) {
	if deps.Accounts == nil || deps.Catalog == nil || deps.Connections == nil || deps.ConnectionDrops == nil || deps.PurchaseOperations == nil || deps.Statistics == nil || deps.Billing == nil || deps.Activity == nil || deps.Coupons == nil || deps.Questionnaires == nil || deps.Emby == nil || deps.EmbyOperations == nil || deps.EmbyPrice == nil || deps.Admin == nil || deps.AdminUsers == nil || deps.Compensation == nil || deps.Settings == nil || deps.PaymentProfiles == nil || deps.Store == nil || deps.Affiliates == nil || deps.Telegram == nil || deps.Webhooks == nil || deps.PublicURL == nil || deps.Logger == nil || len(deps.AdminTelegramIDs) == 0 {
		return nil, errors.New("HTTP API dependencies are incomplete")
	}
	adminTelegramIDs := make(map[int64]struct{}, len(deps.AdminTelegramIDs))
	for _, adminID := range deps.AdminTelegramIDs {
		if adminID <= 0 {
			return nil, errors.New("HTTP API dependencies are incomplete")
		}
		adminTelegramIDs[adminID] = struct{}{}
	}
	requestVerifier, err := requestauth.New(deps.RequestSigningKey)
	if err != nil {
		return nil, err
	}
	server := &Server{deps: deps, requests: requestVerifier, authLimiter: newAuthLimiter(), adminTelegramIDs: adminTelegramIDs}
	if server.deps.APITokens == nil {
		server.deps.APITokens = apitoken.New(deps.Store, deps.AdminTelegramIDs)
	}
	if server.deps.BrowserAuth == nil {
		server.deps.BrowserAuth = browserauth.New(deps.Store, deps.PublicURL, deps.AdminTelegramIDs)
	}
	if server.deps.FirstEntry == nil {
		server.deps.FirstEntry = &accounts.FirstEntry{Repository: deps.Store, Settings: deps.Settings, Hostname: deps.PublicURL.Hostname(), AdminTelegramIDs: deps.AdminTelegramIDs}
	}
	router := chi.NewRouter()
	router.Use(middleware.RealIP)
	router.Use(middleware.RequestID)
	router.Use(server.recoverer)
	router.Use(server.securityHeaders)
	router.Use(server.accessLog)
	router.Use(server.validateRequest)

	router.Get("/healthz", server.health)
	router.Get("/readyz", server.ready)
	router.With(server.limitAuthentication).Post("/api/v1/auth/telegram", server.authenticate)
	server.mountBrowserAuth(router)
	if deps.XNodeMaster != nil {
		router.Get("/v1/agents/connect", deps.XNodeMaster.ServeHTTP)
	}
	if deps.XNode != nil {
		router.Get("/subscription/{token}", server.subscriptionLanding)
		router.Get("/api/v1/subscriptions/{token}/metadata", server.xnodeBearerMetadata)
		router.Get("/api/v1/subscriptions/{token}", server.xnodeBearerSubscription)
	}
	router.Post("/api/v1/webhooks/telegram", server.telegramWebhook)
	router.Get("/api/v1/webhooks/ezpay", server.ezpayWebhook)
	router.Post("/api/v1/webhooks/bepusdt", server.bepusdtWebhook)
	router.Post("/api/v1/webhooks/bepusdt/probe", server.bepusdtProbeWebhook)
	router.Post("/api/v1/webhooks/bepusdt/{capability}", server.bepusdtWebhook)
	router.Get("/api/v1/payments/return/{provider}/{orderID}/status", server.paymentReturnStatus)
	router.Get("/api/v1/payments/return/{provider}", server.paymentReturn)
	router.Get("/api/v1/payments/return/{provider}/{orderID}", server.paymentReturn)

	router.Group(func(authenticated chi.Router) {
		authenticated.Use(server.requireSignedRequest)
		authenticated.Use(server.requireSession)
		authenticated.Use(server.requireCaptcha)
		authenticated.Get("/api/v1/me", server.me)
		authenticated.Post("/api/v1/me/captcha", server.verifyCaptcha)
		authenticated.Get("/api/v1/me/display-currency", server.displayCurrency)
		authenticated.Put("/api/v1/me/display-currency", server.updateDisplayCurrency)
		authenticated.Post("/api/v1/onboarding/continue", server.continueOnboarding)
		authenticated.Post("/api/v1/onboarding/agreement", server.acceptAgreement)
		authenticated.Get("/api/v1/onboarding/content", server.onboardingContent)
		authenticated.Get("/api/v1/dashboard", server.dashboard)
		authenticated.Get("/api/v1/dashboard/node-usage", server.dashboardNodeUsage)
		authenticated.Get("/api/v1/catalog", server.catalog)
		authenticated.Post("/api/v1/purchases/quote", server.purchaseQuote)
		authenticated.Post("/api/v1/purchases", server.purchase)
		authenticated.Get("/api/v1/purchases", server.purchases)
		authenticated.Post("/api/v1/purchases/{id}/addons/quote", server.purchaseAddonQuote)
		authenticated.Post("/api/v1/purchases/{id}/addons", server.purchaseAddons)
		authenticated.Post("/api/v1/purchases/{id}/cancel", server.cancelQueuedPurchase)
		authenticated.Get("/api/v1/purchases/{id}/rollover", server.rolloverProjection)
		authenticated.Get("/api/v1/purchases/{id}/auto-renewal", server.automaticRenewal)
		authenticated.Put("/api/v1/purchases/{id}/auto-renewal", server.updateAutomaticRenewal)
		authenticated.Post("/api/v1/subscription/revoke", server.revokeSubscription)
		server.mountMemberOperations(authenticated)
		server.mountXNodeMembers(authenticated)
		server.mountIPLookup(authenticated)
		server.mountUSDT(authenticated)
		authenticated.Get("/api/v1/balance", server.balance)
		authenticated.Get("/api/v1/ledger", server.ledger)
		authenticated.Get("/api/v1/affiliates", server.affiliateOverview)
		authenticated.Get("/api/v1/affiliates/referrals", server.affiliateReferrals)
		authenticated.Post("/api/v1/payments/orders", server.createPaymentOrder)
		authenticated.Get("/api/v1/payments/orders/{id}", server.paymentOrder)
		authenticated.Post("/api/v1/payments/orders/{id}/cancel", server.cancelPaymentOrder)
		authenticated.Get("/api/v1/emby/account", server.embyAccount)
		authenticated.Post("/api/v1/emby/setup", server.setupEmby)
		authenticated.Put("/api/v1/emby/preferences", server.updateEmbyPreferences)
		authenticated.Put("/api/v1/emby/password", server.changeEmbyPassword)
		server.mountCommunity(authenticated)

		authenticated.Route("/api/v1/admin", func(adminRouter chi.Router) {
			adminRouter.Use(server.requireAdmin)
			if deps.DatabaseAdmin != nil {
				deps.DatabaseAdmin.Mount(adminRouter)
			}
			server.mountAdmin(adminRouter)
			server.mountXNodeAdmin(adminRouter)
		})
	})

	if deps.Static != nil {
		router.Handle("/*", spaHandler(deps.Static))
	}
	server.router = router
	return server, nil
}

// Handler returns the fully configured router.
func (s *Server) Handler() http.Handler { return s.router }
