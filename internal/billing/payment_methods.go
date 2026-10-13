package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

// Methods returns the configured ordered rail list. A method is selectable only
// when both its provider and new-direction rate are configured.
func (s *Service) Methods(ctx context.Context) ([]model.PaymentMethod, error) {
	result := []model.PaymentMethod{{ID: "coupon", Provider: "coupon", Name: "Coupon", Currency: "TXB", Available: true, Mode: "coupon_redemption"}}
	if s.builtin != nil {
		methods, err := s.builtin.Methods(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, methods...)
	}
	if profiles, ok := s.settings.(paymentProfileReader); ok {
		items, err := profiles.PaymentProfiles(ctx)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			for _, profile := range items {
				if profile.Provider != "ezpay" && profile.Provider != "bepusdt" {
					continue
				}
				rate, rateErr := s.loadNewRate(ctx, profile.Provider)
				available := profile.Enabled && profile.Configured && rateErr == nil && rate.Positive()
				channels := paymentChannelsByRail(s.settings, profile.ID)
				for _, rail := range profile.EnabledChannels {
					method := model.PaymentMethod{ID: profile.Provider + ":" + profile.ID + ":" + rail, Provider: profile.Provider, ProfileID: profile.ID, ProviderName: profile.ProviderName, Rail: rail,
						Name: methodName(profile.Provider, rail), Currency: strings.ToUpper(currencyCode(profile.Provider)), Available: available, Mode: "order"}
					if channel, ok := channels[rail]; ok {
						method.CryptoCurrency = channel.Currency
						method.Network = channel.Network
						method.NetworkName = channel.NetworkName
					}
					result = append(result, method)
				}
			}
			starsEnabled, starsErr := s.settings.Optional(ctx, "billing.stars.enabled")
			if starsErr != nil {
				return nil, starsErr
			}
			if starsEnabled == "true" {
				rate, rateErr := s.loadNewRate(ctx, "stars")
				result = append(result, methodModel("stars", "", "", rateErr == nil && rate.Positive(), ""))
			}
			return result, nil
		}
	}
	for _, provider := range []string{"ezpay", "bepusdt", "stars"} {
		enabled, err := s.settings.Optional(ctx, "billing."+provider+".enabled")
		if err != nil {
			return nil, err
		}
		if enabled != "true" {
			continue
		}
		rate, rateErr := s.loadNewRate(ctx, provider)
		available := rateErr == nil && rate.Positive()
		note := ""
		if !available {
			note = "Administrator must enter the TXB rate"
		}
		if provider == "stars" {
			result = append(result, methodModel(provider, "", "", available, note))
			continue
		}
		raw, err := s.settings.Optional(ctx, "billing."+provider+".methods")
		if err != nil {
			return nil, err
		}
		allowed := ezpayRails
		if provider == "bepusdt" {
			allowed = bepusdtRails
		}
		rails, err := parseEnabledRails(raw, allowed)
		if err != nil {
			return nil, fmt.Errorf("load %s methods: %w", provider, err)
		}
		for _, rail := range rails {
			result = append(result, methodModel(provider, "", rail, available, note))
		}
	}
	return result, nil
}

func paymentChannelsByRail(settings Settings, profileID string) map[string]model.PaymentChannel {
	reader, ok := settings.(paymentProfileChannelReader)
	if !ok {
		return nil
	}
	channels := reader.PaymentProfileChannels(profileID)
	result := make(map[string]model.PaymentChannel, len(channels))
	for _, channel := range channels {
		result[channel.Rail] = channel
	}
	return result
}
