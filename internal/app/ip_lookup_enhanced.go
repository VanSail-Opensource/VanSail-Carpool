package app

import (
	"github.com/txyyddss/Remna-User-Panel/internal/admin"
	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

// NewIPLookupEnhanced composes the shared inventories and queued active probes.
func NewIPLookupEnhanced(settings *admin.SettingsService, queues map[string]*upstreamqueue.Queue, live *iplookup.LiveService) *iplookup.EnhancedService {
	return iplookup.NewEnhancedService(settings, queues, live)
}
