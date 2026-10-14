package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type queuedPurchaseCanceller interface {
	CancelQueuedPurchase(context.Context, string, string, string, time.Time) (model.Purchase, error)
}

// CancelQueuedPurchase cancels only the authenticated member's queued
// entitlement and refunds its charged TXB amount atomically.
func (s *Service) CancelQueuedPurchase(ctx context.Context, user model.User, purchaseID string) (model.Purchase, error) {
	if user.OnboardingState != "complete" || strings.TrimSpace(purchaseID) == "" {
		return model.Purchase{}, errors.New("invalid queued purchase cancellation")
	}
	canceller, ok := s.repository.(queuedPurchaseCanceller)
	if !ok {
		return model.Purchase{}, errors.New("queued purchase cancellation is unavailable")
	}
	return canceller.CancelQueuedPurchase(ctx, user.ID, strings.TrimSpace(purchaseID), "Queued entitlement cancelled by member", s.now().UTC())
}
