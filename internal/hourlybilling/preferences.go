package hourlybilling

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Service) currentHourlyOwner(ctx context.Context, user model.User) (model.User, error) {
	if s.store == nil || user.ID == "" || user.AccountGeneration < 0 {
		return model.User{}, ErrUnavailable
	}
	current, err := s.store.UserByID(ctx, user.ID)
	if err != nil {
		return model.User{}, err
	}
	if current.AccountStatus != "active" || current.AccountGeneration != user.AccountGeneration {
		return model.User{}, database.ErrAccountUnavailable
	}
	return current, nil
}

// SetExtraPreference opts out before loading mutable pricing/configuration. New
// opt-in requires explicit consent to the current encrypted policy's price.
func (s *Service) SetExtraPreference(ctx context.Context, user model.User, enabled bool, version string, acceptedPriceMinor *int64) (model.HourlyExtraPreference, error) {
	owner, err := s.currentHourlyOwner(ctx, user)
	if err != nil {
		return model.HourlyExtraPreference{}, err
	}
	input := database.HourlyExtraPreferenceInput{OwnerID: owner.ID, AccountGeneration: owner.AccountGeneration, Enabled: enabled}
	if !enabled {
		return s.store.SetHourlyExtraPreference(ctx, input, model.HourlyPolicySnapshot{}, s.now().UTC())
	}
	if s.policy == nil || acceptedPriceMinor == nil || *acceptedPriceMinor <= 0 {
		return model.HourlyExtraPreference{}, database.ErrConflict
	}
	policy, err := s.policy.HourlyPolicy(ctx)
	if err != nil {
		return model.HourlyExtraPreference{}, err
	}
	if version == "" || version != policy.Version {
		return model.HourlyExtraPreference{}, database.ErrConflict
	}
	input.AcceptedPriceTXBMinor = *acceptedPriceMinor
	return s.store.SetHourlyExtraPreference(ctx, input, policy, s.now().UTC())
}

func (s *Service) hourlyAccountUsage(ctx context.Context, owner model.User, policy model.HourlyPolicy, now time.Time) (*model.HourlyAccountUsage, error) {
	day := now.UTC().Format(time.DateOnly)
	usage := &model.HourlyAccountUsage{DayUTC: day, FreeAllowanceBytes: policy.DailyAllowanceBytes,
		ReducedRateBytesPerSecond: policy.ReducedRateBytesPerSecond,
		ExtraExpiresAt:            now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)}
	value, err := s.store.HourlyCreditDay(ctx, owner.ID, owner.AccountGeneration, day)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		usage.FreeAllowanceBytes, usage.FreeUsedBytes, usage.FreeReservedBytes = value.FreeAllowanceBytes, value.FreeAdmittedBytes, value.FreeReservedBytes
		usage.ExtraFundedBytes, usage.ExtraUsedBytes, usage.ExtraReservedBytes = value.ExtraFundedBytes, value.ExtraAdmittedBytes, value.ExtraReservedBytes
	}
	outstanding, err := s.store.OutstandingHourlyCredits(ctx, owner.ID, owner.AccountGeneration, "", 1)
	if err != nil {
		return nil, err
	}
	usage.AccountingPending = len(outstanding) != 0
	return usage, nil
}
