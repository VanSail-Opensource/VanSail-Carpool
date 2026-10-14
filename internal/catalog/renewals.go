package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type renewalRepository interface {
	RenewalQuote(context.Context, string, string, int, time.Time) (model.RenewalQuote, error)
	Renew(context.Context, database.RenewalInput, time.Time) (model.RenewalBatch, error)
}

type renewalAddonSelectionRepository interface {
	RenewalQuoteExcludingAddons(context.Context, string, string, int, []string, time.Time) (model.RenewalQuote, error)
	RenewExcludingAddons(context.Context, database.RenewalInput, []string, time.Time) (model.RenewalBatch, error)
}

// RenewalQuote previews a contiguous renewal using current prices and any
// valid recurring coupon attached to the source purchase.
func (s *Service) RenewalQuote(ctx context.Context, user model.User, purchaseID string, termCount int) (model.RenewalQuote, error) {
	if user.OnboardingState != "complete" || strings.TrimSpace(purchaseID) == "" || termCount < 1 || termCount > 6 {
		return model.RenewalQuote{}, errors.New("invalid renewal selection")
	}
	quote, _, err := s.renewalQuoteWithLiveAddons(ctx, user.ID, purchaseID, termCount, s.now().UTC())
	return quote, err
}

func (s *Service) renewalQuoteWithLiveAddons(ctx context.Context, userID, purchaseID string, termCount int, now time.Time) (model.RenewalQuote, []string, error) {
	repository, ok := s.repository.(renewalRepository)
	if !ok {
		return model.RenewalQuote{}, nil, errors.New("renewal is unavailable")
	}
	quote, err := repository.RenewalQuote(ctx, userID, purchaseID, termCount, now)
	if err != nil {
		return model.RenewalQuote{}, nil, err
	}
	catalog, unavailable, reason, err := s.renewalCatalog(ctx, quote.ComboID, quote.AddonSquadUUIDs)
	if err != nil {
		return model.RenewalQuote{}, nil, err
	}
	if reason != "" {
		return model.RenewalQuote{}, nil, database.ErrNotFound
	}
	if len(unavailable) > 0 {
		selection, ok := repository.(renewalAddonSelectionRepository)
		if !ok {
			return model.RenewalQuote{}, nil, errors.New("renewal selection is unavailable")
		}
		quote, err = selection.RenewalQuoteExcludingAddons(ctx, userID, purchaseID, termCount, unavailable, now)
		if err != nil {
			return model.RenewalQuote{}, nil, err
		}
	}
	quote.AccessibleNodes = quoteAccessibleNodes(catalog, quote.ComboID, quote.AddonSquadUUIDs)
	if len(quote.AccessibleNodes) == 0 {
		return model.RenewalQuote{}, nil, ErrNoAccessibleNodes
	}
	return quote, unavailable, nil
}

// Renew commits one atomic debit for 1-6 contiguous current-ride terms.
func (s *Service) Renew(ctx context.Context, user model.User, purchaseID string, termCount int, idempotencyKey string) (model.RenewalBatch, error) {
	if user.OnboardingState != "complete" || strings.TrimSpace(purchaseID) == "" || termCount < 1 || termCount > 6 {
		return model.RenewalBatch{}, errors.New("invalid renewal selection")
	}
	if strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 128 {
		return model.RenewalBatch{}, errors.New("invalid idempotency key")
	}
	_, unavailable, err := s.renewalQuoteWithLiveAddons(ctx, user.ID, purchaseID, termCount, s.now().UTC())
	if err != nil {
		return model.RenewalBatch{}, err
	}
	repository, ok := s.repository.(renewalRepository)
	if !ok {
		return model.RenewalBatch{}, errors.New("renewal is unavailable")
	}
	input := database.RenewalInput{UserID: user.ID, PurchaseID: purchaseID, TermCount: termCount, IdempotencyKey: idempotencyKey}
	if len(unavailable) > 0 {
		selection, ok := repository.(renewalAddonSelectionRepository)
		if !ok {
			return model.RenewalBatch{}, errors.New("renewal selection is unavailable")
		}
		return selection.RenewExcludingAddons(ctx, input, unavailable, s.now().UTC())
	}
	return repository.Renew(ctx, input, s.now().UTC())
}
