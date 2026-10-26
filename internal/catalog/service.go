// Package catalog implements catalog browsing, TXB purchases, and dashboard composition.
package catalog

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// ErrNoAccessibleNodes means the selected entitlement cannot be delivered by
// any currently enabled Proxy node.
var ErrNoAccessibleNodes = errors.New("no accessible nodes")

// Repository is the local catalog and entitlement persistence contract.
type Repository interface {
	ListCombos(context.Context, bool) ([]model.Combo, error)
	ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error)
	ListSquadProductOverrides(context.Context, bool) ([]model.SquadProduct, error)
	CreatePurchase(context.Context, database.PurchaseInput, time.Time) (model.Purchase, error)
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

type quoteRepository interface {
	QuotePurchase(context.Context, database.PurchaseInput, time.Time) (model.PurchaseQuote, error)
}

type cachedDashboard struct {
	value     RemoteDashboard
	fetchedAt time.Time
	bytes     int
}

// Service owns price selection and short-lived statistics caching.
type Service struct {
	repository   Repository
	provider     ProxyClient
	cacheTTL     time.Duration
	now          func() time.Time
	cacheMu      sync.RWMutex
	cache        map[string]cachedDashboard
	cacheBytes   int
	connectivity *connectivity.Service
}

// NewService creates a catalog service.
func NewService(repository Repository, provider ProxyClient, cacheTTL time.Duration) *Service {
	return &Service{repository: repository, provider: provider, cacheTTL: cacheTTL, now: time.Now, cache: make(map[string]cachedDashboard)}
}

// Purchase delegates all pricing and balance work to one SQLite transaction.
func (s *Service) Purchase(ctx context.Context, user model.User, comboID string, addonIDs []string, idempotencyKey string) (model.Purchase, error) {
	return s.PurchaseWithCoupon(ctx, user, comboID, addonIDs, "", nil, idempotencyKey)
}

// PurchaseWithCoupon applies at most one explicitly selected wallet grant.
func (s *Service) PurchaseWithCoupon(ctx context.Context, user model.User, comboID string, addonIDs []string, couponGrantID string, activationCodes map[string]string, idempotencyKey string) (model.Purchase, error) {
	if user.OnboardingState != "complete" {
		return model.Purchase{}, errors.New("onboarding is incomplete")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if comboID == "" || len(addonIDs) > 100 || idempotencyKey == "" || len(idempotencyKey) > 128 {
		return model.Purchase{}, errors.New("invalid purchase selection")
	}
	catalog, err := s.validateLiveSelection(ctx, comboID, addonIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	nodes := quoteAccessibleNodes(catalog, comboID, addonIDs)
	if len(nodes) == 0 {
		return model.Purchase{}, ErrNoAccessibleNodes
	}
	return s.repository.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: comboID, AddonSquadIDs: addonIDs,
		CouponGrantID: couponGrantID, SquadActivationCodes: activationCodes, IdempotencyKey: idempotencyKey}, s.now().UTC())
}

// Quote returns the authoritative price and entitlement start time without
// mutating the user's balance, coupon wallet, or purchases.
func (s *Service) Quote(ctx context.Context, user model.User, comboID string, addonIDs []string, couponGrantID string) (model.PurchaseQuote, error) {
	if user.OnboardingState != "complete" {
		return model.PurchaseQuote{}, errors.New("onboarding is incomplete")
	}
	if comboID == "" || len(addonIDs) > 100 {
		return model.PurchaseQuote{}, errors.New("invalid purchase selection")
	}
	catalog, err := s.validateLiveSelection(ctx, comboID, addonIDs)
	if err != nil {
		return model.PurchaseQuote{}, err
	}
	repository, ok := s.repository.(quoteRepository)
	if !ok {
		return model.PurchaseQuote{}, errors.New("purchase quotes are unavailable")
	}
	quote, err := repository.QuotePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: comboID, AddonSquadIDs: addonIDs, CouponGrantID: couponGrantID}, s.now().UTC())
	if err != nil {
		return model.PurchaseQuote{}, err
	}
	quote.AccessibleNodes = quoteAccessibleNodes(catalog, comboID, addonIDs)
	if len(quote.AccessibleNodes) == 0 {
		return model.PurchaseQuote{}, ErrNoAccessibleNodes
	}
	return quote, nil
}

func (s *Service) validateLiveSelection(ctx context.Context, comboID string, addonIDs []string) (model.Catalog, error) {
	if _, ok := s.provider.(remoteSquadLister); !ok {
		return s.Catalog(ctx)
	}
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return model.Catalog{}, err
	}
	comboFound := false
	for _, combo := range catalog.Combos {
		if combo.ID == comboID {
			comboFound = true
			break
		}
	}
	if !comboFound {
		return model.Catalog{}, database.ErrNotFound
	}
	visible := make(map[string]struct{}, len(catalog.Addons))
	for _, addon := range catalog.Addons {
		visible[addon.GroupID] = struct{}{}
	}
	for _, addonID := range addonIDs {
		if _, exists := visible[strings.TrimSpace(addonID)]; !exists {
			return model.Catalog{}, database.ErrNotFound
		}
	}
	return catalog, nil
}

// Purchases returns the account's immutable purchase history.
func (s *Service) Purchases(ctx context.Context, userID string) ([]model.Purchase, error) {
	return s.repository.ListPurchases(ctx, userID)
}
