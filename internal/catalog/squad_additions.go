package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type purchaseAddonRepository interface {
	QuotePurchaseAddons(context.Context, database.PurchaseAddonInput, time.Time) (model.PurchaseAddonQuote, error)
	AddPurchaseAddons(context.Context, database.PurchaseAddonInput, time.Time) (model.Purchase, error)
}
type purchaseAddonReplayRepository interface {
	PurchaseAddonReplay(context.Context, database.PurchaseAddonInput) (model.Purchase, bool, error)
}

// QuoteAddons returns an owned active purchase's authoritative addition price.
func (s *Service) QuoteAddons(ctx context.Context, user model.User, purchaseID string, addonIDs []string, coupon ...string) (model.PurchaseAddonQuote, error) {
	if user.OnboardingState != "complete" {
		return model.PurchaseAddonQuote{}, errors.New("onboarding is incomplete")
	}
	if err := s.validateLiveAddons(ctx, addonIDs); err != nil {
		return model.PurchaseAddonQuote{}, err
	}
	repository, ok := s.repository.(purchaseAddonRepository)
	if !ok {
		return model.PurchaseAddonQuote{}, errors.New("purchase add-ons are unavailable")
	}
	return repository.QuotePurchaseAddons(ctx, database.PurchaseAddonInput{UserID: user.ID, PurchaseID: purchaseID, CouponGrantID: firstAddonCoupon(coupon), AddonSquadIDs: addonIDs}, s.now().UTC())
}

// AddAddons applies an owned active purchase's selected optional squads.
func (s *Service) AddAddons(ctx context.Context, user model.User, purchaseID string, addonIDs []string, activationCodes map[string]string, idempotencyKey string, coupon ...string) (model.Purchase, error) {
	return s.AddAddonsWithQuote(ctx, user, purchaseID, addonIDs, activationCodes, idempotencyKey, "", coupon...)
}

// AddAddonsWithQuote requires the reviewed amount for dynamically priced additions.
func (s *Service) AddAddonsWithQuote(ctx context.Context, user model.User, purchaseID string, addonIDs []string, activationCodes map[string]string, idempotencyKey, quoteVersion string, coupon ...string) (model.Purchase, error) {
	if user.OnboardingState != "complete" {
		return model.Purchase{}, errors.New("onboarding is incomplete")
	}
	if replay, ok := s.repository.(purchaseAddonReplayRepository); ok {
		purchase, found, err := replay.PurchaseAddonReplay(ctx, database.PurchaseAddonInput{UserID: user.ID, PurchaseID: purchaseID, CouponGrantID: firstAddonCoupon(coupon), AddonSquadIDs: addonIDs, SquadActivationCodes: activationCodes, IdempotencyKey: idempotencyKey})
		if found || err != nil {
			return purchase, err
		}
	}
	if err := s.validateLiveAddons(ctx, addonIDs); err != nil {
		return model.Purchase{}, err
	}
	repository, ok := s.repository.(purchaseAddonRepository)
	if !ok {
		return model.Purchase{}, errors.New("purchase add-ons are unavailable")
	}
	return repository.AddPurchaseAddons(ctx, database.PurchaseAddonInput{QuoteVersion: quoteVersion, UserID: user.ID, PurchaseID: purchaseID, CouponGrantID: firstAddonCoupon(coupon),
		AddonSquadIDs: addonIDs, SquadActivationCodes: activationCodes, IdempotencyKey: idempotencyKey}, s.now().UTC())
}

func (s *Service) validateLiveAddons(ctx context.Context, addonIDs []string) error {
	if len(addonIDs) == 0 || len(addonIDs) > 100 {
		return errors.New("invalid squad selection")
	}
	addons, err := s.repository.ListSquadProducts(ctx, true)
	if err != nil {
		return err
	}
	if _, live := s.provider.(remoteSquadLister); live {
		catalog, err := s.hydrateLiveCatalog(ctx, nil, addons)
		if err != nil {
			return err
		}
		addons = catalog.Addons
	}
	visible := make(map[string]struct{}, len(addons))
	for _, addon := range addons {
		if addon.Visible && addon.UpstreamPresent {
			visible[addon.GroupID] = struct{}{}
		}
	}
	for _, addonID := range addonIDs {
		if _, found := visible[strings.TrimSpace(addonID)]; !found {
			return database.ErrNotFound
		}
	}
	return nil
}

func firstAddonCoupon(coupon []string) string {
	if len(coupon) == 1 {
		return strings.TrimSpace(coupon[0])
	}
	return ""
}
