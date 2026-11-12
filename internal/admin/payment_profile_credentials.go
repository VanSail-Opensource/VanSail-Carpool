package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// ErrPaymentProfileFreshCredential requires an explicit credential when an
// existing profile is retargeted to a different provider account or endpoint.
var ErrPaymentProfileFreshCredential = errors.New("changing the payment account requires a replacement credential")

// ErrPaymentProfileSettings protects the retained legacy settings once account
// profiles own payment credentials and endpoint configuration.
var ErrPaymentProfileSettings = errors.New("legacy payment account settings require the dedicated payment profile interface")

func (s *SettingsService) validateLegacyPaymentSettingWrite(ctx context.Context, key string) error {
	provider := ""
	switch key {
	case "billing.ezpay.base_url", "billing.ezpay.merchant_id", "billing.ezpay.key":
		provider = "ezpay"
	case "billing.bepusdt.base_url", "billing.bepusdt.api_token":
		provider = "bepusdt"
	}
	if provider == "" || s.profiles == nil {
		return nil
	}
	profiles, err := s.profiles.ListPaymentProfiles(ctx)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if profile.Provider == provider {
			return ErrPaymentProfileSettings
		}
	}
	return nil
}

func (s *SettingsService) validateRetainedPaymentCredential(ctx context.Context, id string, input model.PaymentProfile) error {
	profiles, err := s.profiles.ListPaymentProfiles(ctx)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if profile.ID == id && (profile.Provider != input.Provider || profile.Endpoint != strings.TrimSpace(input.Endpoint) || profile.MerchantID != strings.TrimSpace(input.MerchantID)) {
			return ErrPaymentProfileFreshCredential
		}
	}
	return nil
}

func (s *SettingsService) paymentProfileCredential(ctx context.Context, record database.PaymentProfileRecord) (string, error) {
	plaintext, err := s.vault.Decrypt("payment-profile:"+record.ID, record.CredentialCiphertext)
	if err == nil {
		return plaintext, nil
	}
	// The profile migration copied only the original built-in provider rows.
	// Accept their old AAD only when the retained encrypted setting proves that
	// this exact ciphertext was copied. A corrupt/custom account must never
	// inherit a different account's global credential.
	legacyKey := ""
	if record.ID == record.Provider {
		switch record.Provider {
		case "ezpay":
			legacyKey = "billing.ezpay.key"
		case "bepusdt":
			legacyKey = "billing.bepusdt.api_token"
		}
	}
	if legacyKey != "" && record.CredentialCiphertext != "" {
		setting, settingErr := s.repository.GetSetting(ctx, legacyKey)
		if settingErr == nil && setting.Encrypted && setting.Value == record.CredentialCiphertext {
			plaintext, legacyErr := s.vault.Decrypt(legacyKey, record.CredentialCiphertext)
			if legacyErr == nil {
				return plaintext, nil
			}
		}
	}
	return "", fmt.Errorf("decrypt payment profile %q: %w", record.ID, err)
}
