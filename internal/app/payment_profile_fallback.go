package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/ezpay"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// Legacy settings are usable only when no account profile exists. A disabled
// rail, corrupt credential or unavailable repository cannot select a different
// account implicitly.
func (a paymentAdapter) legacyPaymentFallback(ctx context.Context, provider string, lookupErr error) error {
	if !errors.Is(lookupErr, database.ErrNotFound) {
		return lookupErr
	}
	profiles, err := a.settings.PaymentProfiles(ctx)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if profile.Provider == provider {
			return lookupErr
		}
	}
	return nil
}

func (a paymentAdapter) ezpayClient(ctx context.Context, profileID, rail string) (*ezpay.Client, error) {
	var profile model.PaymentProfileRuntime
	var err error
	if profileID != "" {
		profile, err = a.settings.PaymentProfileByID(ctx, profileID, rail)
	} else {
		profile, err = a.settings.PaymentProfile(ctx, "ezpay", rail)
	}
	if err != nil {
		if profileID != "" {
			return nil, err
		}
		if fallbackErr := a.legacyPaymentFallback(ctx, "ezpay", err); fallbackErr != nil {
			return nil, fallbackErr
		}
		baseURL, fallbackErr := a.settings.Plaintext(ctx, "billing.ezpay.base_url")
		if fallbackErr != nil {
			return nil, err
		}
		merchantID, merchantErr := a.settings.Plaintext(ctx, "billing.ezpay.merchant_id")
		if merchantErr != nil {
			return nil, merchantErr
		}
		key, keyErr := a.settings.Plaintext(ctx, "billing.ezpay.key")
		if keyErr != nil {
			return nil, keyErr
		}
		return ezpay.NewClient(baseURL, merchantID, key)
	}
	if profile.Provider != "ezpay" {
		return nil, fmt.Errorf("payment profile %q belongs to %q, not ezpay", profileID, profile.Provider)
	}
	return ezpay.NewClient(profile.Endpoint, profile.MerchantID, profile.CredentialPlaintext)
}
