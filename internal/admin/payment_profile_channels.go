package admin

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type paymentProfileChannelReader interface {
	PaymentProfileChannels(string, time.Time) []model.PaymentChannel
	PaymentProfileRails(string, time.Time) []string
}

type paymentProfileMutationRepository interface {
	DeletePaymentProfile(context.Context, string, string) error
}

type paymentProfileAvailabilityRepository interface {
	DisablePaymentProfileIfUnchanged(context.Context, string, time.Time) (model.PaymentProfile, bool, error)
}

// SetPaymentProfileChannels attaches the process-local discovery source.
func (s *SettingsService) SetPaymentProfileChannels(reader paymentProfileChannelReader) {
	s.channels = reader
}

// PaymentProfileChannels returns the process-local discovery snapshot used by member billing.
func (s *SettingsService) PaymentProfileChannels(profileID string, updatedAt time.Time) []model.PaymentChannel {
	if s.channels == nil {
		return nil
	}
	return s.channels.PaymentProfileChannels(profileID, updatedAt)
}

func (s *SettingsService) applyDiscoveredChannels(profile *model.PaymentProfile) {
	if profile == nil || profile.Provider != "bepusdt" {
		return
	}
	profile.EnabledChannels = nil
	if s.channels != nil {
		profile.EnabledChannels = s.channels.PaymentProfileRails(profile.ID, profile.UpdatedAt)
	}
}

// DisablePaymentProfileIfUnchanged disables only the revision that was probed.
func (s *SettingsService) DisablePaymentProfileIfUnchanged(ctx context.Context, id string, updatedAt time.Time) (model.PaymentProfile, bool, error) {
	repository, ok := s.profiles.(paymentProfileAvailabilityRepository)
	if !ok {
		return model.PaymentProfile{}, false, errors.New("payment profile mutation is unavailable")
	}
	profile, disabled, err := repository.DisablePaymentProfileIfUnchanged(ctx, id, updatedAt)
	s.applyDiscoveredChannels(&profile)
	return profile, disabled, err
}

// DeletePaymentProfile removes a profile that has no open checkout attempts.
func (s *SettingsService) DeletePaymentProfile(ctx context.Context, id, actorID string) error {
	repository, ok := s.profiles.(paymentProfileMutationRepository)
	if !ok {
		return errors.New("payment profile mutation is unavailable")
	}
	return repository.DeletePaymentProfile(ctx, id, actorID)
}
