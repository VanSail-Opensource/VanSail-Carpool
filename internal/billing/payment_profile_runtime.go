package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func callbackCapability(secret, orderID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("bepusdt-callback\x00" + orderID))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyBEPusdtCallbackCapability authenticates unsigned v1.19-style callback
// URLs without exposing the configured API token.
func (s *Service) VerifyBEPusdtCallbackCapability(ctx context.Context, orderID, capability string) bool {
	_, valid := s.BEPusdtCallbackProfile(ctx, orderID, capability)
	return valid
}

// BEPusdtCallbackProfile validates an unsigned callback capability and returns
// the provider profile selected by the durable order.
func (s *Service) BEPusdtCallbackProfile(ctx context.Context, orderID, capability string) (string, bool) {
	order, err := s.repository.PaymentOrderByID(ctx, orderID)
	if err != nil {
		return "", false
	}
	profileID := ""
	if order.MethodID != "" {
		provider, parsedProfileID, _, parseErr := ParseMethodSelection(order.MethodID)
		if parseErr != nil || provider != "bepusdt" {
			return "", false
		}
		profileID = parsedProfileID
	}
	secret, err := s.providerCredential(ctx, "bepusdt", profileID)
	if err != nil {
		return "", false
	}
	expected := callbackCapability(secret, orderID)
	if len(capability) != len(expected) || !hmac.Equal([]byte(capability), []byte(expected)) {
		return "", false
	}
	return profileID, true
}

func (s *Service) providerMethodEnabled(ctx context.Context, provider, profileID, rail string) (bool, error) {
	if provider == "builtin_usdt" {
		return s.builtinEnabled(ctx, rail)
	}
	if reader, ok := s.settings.(paymentProfileReader); ok {
		profiles, err := reader.PaymentProfiles(ctx)
		if err != nil {
			return false, err
		}
		if len(profiles) > 0 && provider != "stars" {
			for _, profile := range profiles {
				if profile.Provider == provider && (profileID == "" || profile.ID == profileID) {
					return profile.Enabled && profile.Configured && containsRail(profile.EnabledChannels, rail), nil
				}
			}
			return false, nil
		}
	}
	enabled, err := s.settings.Optional(ctx, "billing."+provider+".enabled")
	if err != nil || enabled != "true" {
		return false, err
	}
	if provider == "stars" {
		return true, nil
	}
	raw, err := s.settings.Optional(ctx, "billing."+provider+".methods")
	if err != nil {
		return false, err
	}
	allowed := ezpayRails
	if provider == "bepusdt" {
		allowed = bepusdtRails
	}
	enabledRails, parseErr := parseEnabledRails(raw, allowed)
	return parseErr == nil && containsRail(enabledRails, rail), nil
}

func (s *Service) providerCredential(ctx context.Context, provider, profileID string) (string, error) {
	if profileID != "" {
		if reader, ok := s.settings.(paymentProfileByIDRuntimeReader); ok {
			profile, err := reader.PaymentProfileByID(ctx, profileID, "")
			if err != nil {
				return "", err
			}
			if profile.Provider != provider {
				return "", fmt.Errorf("payment profile %q belongs to %q, not %s", profileID, profile.Provider, provider)
			}
			if profile.CredentialPlaintext != "" {
				return profile.CredentialPlaintext, nil
			}
		}
	} else if reader, ok := s.settings.(paymentProfileRuntimeReader); ok {
		profile, err := reader.PaymentProfile(ctx, provider, "")
		if err == nil && profile.CredentialPlaintext != "" {
			return profile.CredentialPlaintext, nil
		}
	}
	key := "billing." + provider + ".api_token"
	if provider == "ezpay" {
		key = "billing.ezpay.key"
	}
	return s.settings.Plaintext(ctx, key)
}
