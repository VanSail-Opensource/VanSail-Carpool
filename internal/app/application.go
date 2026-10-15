package app

import (
	"log/slog"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/maintenance"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Application owns all context-managed process resources.
type Application struct {
	usdt            *usdt.Service
	xnodeService    *xnode.Service
	xnodeMaster     *xnode.Master
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
	notifications   *notifications.Scanner
	upstreams       *providerQueues
	paymentProfiles *PaymentProfileManager
}
