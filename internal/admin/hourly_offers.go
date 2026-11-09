package admin

import (
	"context"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type hourlyOfferRepository interface {
	HourlyOffer(context.Context, string) (model.HourlyOffer, error)
	ListHourlyOffers(context.Context) ([]model.HourlyOffer, error)
	SaveHourlyOffer(context.Context, string, model.HourlyOffer, int64) (model.HourlyOffer, error)
}

// HourlyOffers lists explicit hourly merchandising independently of legacy plans.
func (s *Service) HourlyOffers(ctx context.Context) ([]model.HourlyOffer, error) {
	repo, ok := s.repository.(hourlyOfferRepository)
	if !ok {
		return nil, ErrHourlyPolicyUnavailable
	}
	return repo.ListHourlyOffers(ctx)
}

// HourlyOffer reads one explicitly configured squad price and revision.
func (s *Service) HourlyOffer(ctx context.Context, groupID string) (model.HourlyOffer, error) {
	repo, ok := s.repository.(hourlyOfferRepository)
	if !ok {
		return model.HourlyOffer{}, ErrHourlyPolicyUnavailable
	}
	return repo.HourlyOffer(ctx, groupID)
}

// SaveHourlyOffer validates threshold arithmetic against the current policy.
// Price/revision and its audit commit atomically through the dedicated repository.
func (s *Service) SaveHourlyOffer(ctx context.Context, actor string, next model.HourlyOffer, expectedRevision int64) (model.HourlyOffer, error) {
	if next.GroupID == "" || len(next.GroupID) > 128 || strings.TrimSpace(next.GroupID) != next.GroupID || expectedRevision < 0 ||
		next.HourlyPriceTXBMinor < 0 || next.RequiredHours < 0 || next.RequiredHours > model.MaxHourlyRequiredHours ||
		next.Enabled && next.HourlyPriceTXBMinor == 0 {
		return model.HourlyOffer{}, model.ErrHourlyPolicyInvalid
	}
	repo, ok := s.repository.(hourlyOfferRepository)
	if !ok {
		return model.HourlyOffer{}, ErrHourlyPolicyUnavailable
	}
	policy, err := s.HourlyPolicy(ctx)
	if err != nil {
		return model.HourlyOffer{}, err
	}
	if next.HourlyPriceTXBMinor > 0 {
		if _, err = model.HourlyActivationMinimum(policy.Policy, next.RequiredHours, next.HourlyPriceTXBMinor); err != nil {
			return model.HourlyOffer{}, err
		}
	}
	return repo.SaveHourlyOffer(ctx, actor, next, expectedRevision)
}
