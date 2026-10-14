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

// QuoteAddons returns an owned active purchase's authoritative addition price.
func (s *Service) QuoteAddons(ctx context.Context, user model.User, purchaseID string, addonIDs []string) (model.PurchaseAddonQuote, error) {
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
	return repository.QuotePurchaseAddons(ctx, database.PurchaseAddonInput{UserID: user.ID, PurchaseID: purchaseID, AddonSquadIDs: addonIDs}, s.now().UTC())
}

// AddAddons applies an owned active purchase's selected optional squads.
func (s *Service) AddAddons(ctx context.Context, user model.User, purchaseID string, addonIDs []string, activationCodes map[string]string, idempotencyKey string) (model.Purchase, error) {
	if user.OnboardingState != "complete" {
		return model.Purchase{}, errors.New("onboarding is incomplete")
	}
	if err := s.validateLiveAddons(ctx, addonIDs); err != nil {
		return model.Purchase{}, err
	}
	repository, ok := s.repository.(purchaseAddonRepository)
	if !ok {
		return model.Purchase{}, errors.New("purchase add-ons are unavailable")
	}
	return repository.AddPurchaseAddons(ctx, database.PurchaseAddonInput{UserID: user.ID, PurchaseID: purchaseID,
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
	visible := make(map[string]struct{}, len(addons))
	for _, addon := range addons {
		if addon.Visible && addon.UpstreamPresent {
			visible[addon.RemnaSquadUUID] = struct{}{}
		}
	}
	for _, addonID := range addonIDs {
		if _, found := visible[strings.TrimSpace(addonID)]; !found {
			return database.ErrNotFound
		}
	}
	return nil
}
