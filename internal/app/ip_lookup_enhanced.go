package app

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

// NewIPLookupEnhanced composes the shared inventories and queued active probes.
func NewIPLookupEnhanced(settings *admin.SettingsService, queues map[string]*upstreamqueue.Queue, live *iplookup.LiveService) *iplookup.EnhancedService {
	return iplookup.NewEnhancedService(settings, queues, live)
}
