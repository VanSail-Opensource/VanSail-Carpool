package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
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

func (s *Service) providerMethodSelection(ctx context.Context, provider, profileID, rail string) (string, bool, error) {
	if provider == "builtin_usdt" {
		enabled, err := s.builtinEnabled(ctx, rail)
		return "", enabled, err
	}
	if reader, ok := s.settings.(paymentProfileReader); ok {
		profiles, err := reader.PaymentProfiles(ctx)
		if err != nil {
			return "", false, err
		}
		if len(profiles) > 0 && provider != "stars" {
			for _, profile := range profiles {
				if profile.Provider == provider && (profileID == "" || profile.ID == profileID) {
					// Persist the selected account even for a legacy two-part request.
					// A later default-profile change must not retarget queued work.
					return profile.ID, profile.Enabled && profile.Configured && containsRail(profile.EnabledChannels, rail), nil
				}
			}
			return "", false, nil
		}
	}
	if profileID != "" {
		return "", false, nil
	}
	enabled, err := s.settings.Optional(ctx, "billing."+provider+".enabled")
	if err != nil || enabled != "true" {
		return "", false, err
	}
	if provider == "stars" {
		return "", true, nil
	}
	raw, err := s.settings.Optional(ctx, "billing."+provider+".methods")
	if err != nil {
		return "", false, err
	}
	allowed := ezpayRails
	if provider == "bepusdt" {
		allowed = bepusdtRails
	}
	enabledRails, parseErr := parseEnabledRails(raw, allowed)
	return "", parseErr == nil && containsRail(enabledRails, rail), nil
}

func (s *Service) providerCredential(ctx context.Context, provider, profileID string) (string, error) {
	if reader, ok := s.settings.(paymentProfileByIDRuntimeReader); ok {
		lookupID := profileID
		if lookupID == "" {
			lookupID = provider
		}
		profile, err := reader.PaymentProfileByID(ctx, lookupID, "")
		if err == nil {
			if profile.ID != lookupID || profile.Provider != provider || profile.CredentialPlaintext == "" {
				return "", fmt.Errorf("credential for payment profile %q is unavailable", lookupID)
			}
			return profile.CredentialPlaintext, nil
		}
		if profileID != "" || !errors.Is(err, database.ErrNotFound) {
			return "", err
		}
		// Unscoped historical orders belong to the original provider account,
		// whose settings predate provider-named migration profiles.
	} else if profileID != "" {
		return "", fmt.Errorf("credential for payment profile %q is unavailable", profileID)
	} else if reader, ok := s.settings.(paymentProfileRuntimeReader); ok {
		profile, err := reader.PaymentProfile(ctx, provider, "")
		if err == nil {
			if profile.Provider == provider && profile.CredentialPlaintext != "" {
				return profile.CredentialPlaintext, nil
			}
			return "", fmt.Errorf("credential for payment provider %q is unavailable", provider)
		}
		if !errors.Is(err, database.ErrNotFound) {
			return "", err
		}
	}
	key := "billing." + provider + ".api_token"
	if provider == "ezpay" {
		key = "billing.ezpay.key"
	}
	credential, err := s.settings.Plaintext(ctx, key)
	if err != nil {
		return "", err
	}
	if credential == "" {
		return "", fmt.Errorf("credential for payment provider %q is unavailable", provider)
	}
	return credential, nil
}
