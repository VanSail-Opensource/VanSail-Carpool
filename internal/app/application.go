package app

import (
	"log/slog"
	"net/http"

	"github.com/txyyddss/Remna-User-Panel/internal/abuse"
	"github.com/txyyddss/Remna-User-Panel/internal/admin"
	"github.com/txyyddss/Remna-User-Panel/internal/affiliates"
	"github.com/txyyddss/Remna-User-Panel/internal/billing"
	"github.com/txyyddss/Remna-User-Panel/internal/catalog"
	"github.com/txyyddss/Remna-User-Panel/internal/compensation"
	"github.com/txyyddss/Remna-User-Panel/internal/connectivity"
	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/maintenance"
	"github.com/txyyddss/Remna-User-Panel/internal/notifications"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/backup"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/config"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/outbox"
	productstats "github.com/txyyddss/Remna-User-Panel/internal/statistics"
)

// Application owns all context-managed process resources.
type Application struct {
	config          config.Config
	logger          *slog.Logger
	httpServer      *http.Server
	store           *database.Store
	outbox          *outbox.Worker
	backups         *backup.Service
	maintenance     *maintenance.Service
	telegram        *queuedTelegram
	settings        *admin.SettingsService
	catalog         *catalog.Service
	billing         *billing.Service
	statistics      *productstats.Service
	connectivity    *connectivity.Service
	ipLookup        *iplookup.Worker
	compensation    *compensation.Service
	affiliates      *affiliates.Service
	abuse           *abuse.Service
	notifications   *notifications.Scanner
	upstreams       *providerQueues
	paymentProfiles *PaymentProfileManager
}
