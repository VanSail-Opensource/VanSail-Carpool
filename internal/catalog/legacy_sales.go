package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// ErrLegacySalesClosed excludes new fixed-term purchases, not owned renewals.
var ErrLegacySalesClosed = errors.New("new fixed-term purchases are closed")

// ReplayLegacyPurchase preserves uncertain historical checkout responses without
// admitting a new purchase. Internal reward and renewal entry points are separate.
func (s *Service) ReplayLegacyPurchase(ctx context.Context, user model.User, input database.PurchaseInput) (model.Purchase, error) {
	input.UserID = user.ID
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if user.OnboardingState != "complete" || input.ComboID == "" || len(input.AddonSquadIDs) > 100 || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		return model.Purchase{}, ErrLegacySalesClosed
	}
	if replay, ok := s.repository.(interface {
		PurchaseReplay(context.Context, database.PurchaseInput) (model.Purchase, bool, error)
	}); ok {
		if purchase, found, err := replay.PurchaseReplay(ctx, input); found || err != nil {
			return purchase, err
		}
	}
	return model.Purchase{}, ErrLegacySalesClosed
}
