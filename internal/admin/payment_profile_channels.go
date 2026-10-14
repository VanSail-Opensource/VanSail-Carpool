package admin

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type paymentProfileChannelReader interface {
	PaymentProfileChannels(string) []model.PaymentChannel
	PaymentProfileRails(string) []string
}

type paymentProfileMutationRepository interface {
	SetPaymentProfileEnabled(context.Context, string, bool) (model.PaymentProfile, error)
	DeletePaymentProfile(context.Context, string) error
}

// SetPaymentProfileChannels attaches the process-local discovery source.
func (s *SettingsService) SetPaymentProfileChannels(reader paymentProfileChannelReader) {
	s.channels = reader
}

// PaymentProfileChannels returns the process-local discovery snapshot used by member billing.
func (s *SettingsService) PaymentProfileChannels(profileID string) []model.PaymentChannel {
	if s.channels == nil {
		return nil
	}
	return s.channels.PaymentProfileChannels(profileID)
}

func (s *SettingsService) applyDiscoveredChannels(profile *model.PaymentProfile) {
	if profile == nil || profile.Provider != "bepusdt" {
		return
	}
	profile.EnabledChannels = nil
	if s.channels != nil {
		profile.EnabledChannels = s.channels.PaymentProfileRails(profile.ID)
	}
}

// DisablePaymentProfile atomically removes a failed profile from member use.
func (s *SettingsService) DisablePaymentProfile(ctx context.Context, id string) (model.PaymentProfile, error) {
	repository, ok := s.profiles.(paymentProfileMutationRepository)
	if !ok {
		return model.PaymentProfile{}, errors.New("payment profile mutation is unavailable")
	}
	profile, err := repository.SetPaymentProfileEnabled(ctx, id, false)
	s.applyDiscoveredChannels(&profile)
	return profile, err
}

// DeletePaymentProfile removes a profile that has no open checkout attempts.
func (s *SettingsService) DeletePaymentProfile(ctx context.Context, id string) error {
	repository, ok := s.profiles.(paymentProfileMutationRepository)
	if !ok {
		return errors.New("payment profile mutation is unavailable")
	}
	return repository.DeletePaymentProfile(ctx, id)
}
