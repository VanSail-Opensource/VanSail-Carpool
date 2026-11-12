package catalog

// Closed member checkout and direct credential mutation survive only as
// historical regression fixtures. Production entry points use exact purchase
// replay and durable provider operation receipts.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type legacyPurchaseRepository interface {
	CreatePurchase(context.Context, database.PurchaseInput, time.Time) (model.Purchase, error)
}

type quoteRepository interface {
	QuotePurchase(context.Context, database.PurchaseInput, time.Time) (model.PurchaseQuote, error)
}

// Purchase delegates all pricing and balance work to one PostgreSQL transaction.
func (s *Service) Purchase(ctx context.Context, user model.User, comboID string, addonIDs []string, idempotencyKey string) (model.Purchase, error) {
	return s.PurchaseWithCoupon(ctx, user, comboID, addonIDs, "", nil, idempotencyKey)
}

// PurchaseWithCoupon applies at most one explicitly selected wallet grant.
func (s *Service) PurchaseWithCoupon(ctx context.Context, user model.User, comboID string, addonIDs []string, couponGrantID string, activationCodes map[string]string, idempotencyKey string, quoteVersion ...string) (model.Purchase, error) {
	if user.OnboardingState != "complete" {
		return model.Purchase{}, errors.New("onboarding is incomplete")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if comboID == "" || len(addonIDs) > 100 || idempotencyKey == "" || len(idempotencyKey) > 128 {
		return model.Purchase{}, errors.New("invalid purchase selection")
	}
	input := database.PurchaseInput{UserID: user.ID, ComboID: comboID, AddonSquadIDs: addonIDs, CouponGrantID: couponGrantID, SquadActivationCodes: activationCodes, IdempotencyKey: idempotencyKey}
	if len(quoteVersion) > 0 {
		input.QuoteVersion = quoteVersion[0]
	}
	if replay, ok := s.repository.(interface {
		PurchaseReplay(context.Context, database.PurchaseInput) (model.Purchase, bool, error)
	}); ok {
		if p, found, e := replay.PurchaseReplay(ctx, input); found || e != nil {
			return p, e
		}
	}
	catalog, err := s.validateLiveSelection(ctx, comboID, addonIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	nodes := quoteAccessibleNodes(catalog, comboID, addonIDs)
	if len(nodes) == 0 {
		return model.Purchase{}, ErrNoAccessibleNodes
	}
	return s.repository.(legacyPurchaseRepository).CreatePurchase(ctx, input, s.now().UTC())
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

// RevokeSubscription rotates the bearer URL and invalidates the process-local cache.
func (s *Service) RevokeSubscription(ctx context.Context, user model.User) (string, error) {
	if user.ProxyUserID == nil {
		return "", database.ErrNotFound
	}
	url, err := s.provider.RevokeSubscription(ctx, *user.ProxyUserID)
	if err != nil {
		return "", err
	}
	s.InvalidateDashboard(user.ID)
	return url, nil
}
