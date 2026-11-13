// Package catalog implements catalog browsing, owned commerce, and dashboard composition.
package catalog

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ErrNoAccessibleNodes means the selected entitlement cannot be delivered by
// any currently enabled Proxy node.
var ErrNoAccessibleNodes = errors.New("no accessible nodes")

// Repository is the local catalog and entitlement persistence contract.
type Repository interface {
	ListCombos(context.Context, bool) ([]model.Combo, error)
	ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error)
	ListSquadProductOverrides(context.Context, bool) ([]model.SquadProduct, error)
	ListPurchases(context.Context, string) ([]model.Purchase, error)
	Balance(context.Context, string) (model.Money, error)
	ActiveAndQueuedPurchases(context.Context, string, time.Time) (*model.Purchase, *model.Purchase, error)
}

// RemoteDashboard is the safe, normalized Proxy dashboard response.
type RemoteDashboard struct {
	Statistics      model.Statistics
	SubscriptionURL string
}

// ProxyClient fetches user state and revokes subscription credentials.
type ProxyClient interface {
	Dashboard(context.Context, string) (RemoteDashboard, error)
	RevokeSubscription(context.Context, string) (string, error)
}

type cachedDashboard struct {
	value     RemoteDashboard
	fetchedAt time.Time
	bytes     int
	scope     dashboardScope
}

type dashboardScope struct {
	generation int64
	proxyID    string
}

// Service owns price selection and short-lived statistics caching.
type Service struct {
	repository Repository
	provider   ProxyClient
	cacheTTL   time.Duration
	now        func() time.Time
	cacheMu    sync.RWMutex
	cache      map[string]cachedDashboard
	cacheBytes int
	cacheEpoch uint64
}

// NewService creates a catalog service.
func NewService(repository Repository, provider ProxyClient, cacheTTL time.Duration) *Service {
	return &Service{repository: repository, provider: provider, cacheTTL: cacheTTL, now: time.Now, cache: make(map[string]cachedDashboard)}
}

// Purchases returns the account's immutable purchase history.
func (s *Service) Purchases(ctx context.Context, userID string) ([]model.Purchase, error) {
	return s.repository.ListPurchases(ctx, userID)
}
