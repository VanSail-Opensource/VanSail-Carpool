package app

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// callbackPaymentProfile uses an untrusted order identifier only to select the
// credential. The caller must still authenticate the entire provider payload.
// Missing orders retain the all-profile verification path for tombstone replay;
// settlement separately requires matching retained callback evidence.
func (a paymentAdapter) callbackPaymentProfile(ctx context.Context, provider, orderID string) (model.PaymentProfileRuntime, string, bool, error) {
	if a.users == nil {
		return model.PaymentProfileRuntime{}, "", false, errors.New("payment order lookup is unavailable")
	}
	order, err := a.users.PaymentOrderByID(ctx, orderID)
	if errors.Is(err, database.ErrNotFound) {
		return model.PaymentProfileRuntime{}, "", false, nil
	}
	if err != nil {
		return model.PaymentProfileRuntime{}, "", false, err
	}
	if order.Provider != provider {
		return model.PaymentProfileRuntime{}, "", true, database.ErrConflict
	}
	profileID := ""
	if order.MethodID != "" {
		methodProvider, selectedID, _, parseErr := billing.ParseMethodSelection(order.MethodID)
		if parseErr != nil || methodProvider != provider {
			return model.PaymentProfileRuntime{}, "", true, database.ErrConflict
		}
		profileID = selectedID
	}
	lookupID := profileID
	if lookupID == "" {
		// The migration copied each legacy account to its provider-named ID.
		lookupID = provider
	}
	profile, err := a.settings.PaymentProfileByID(ctx, lookupID, "")
	if errors.Is(err, database.ErrNotFound) && profileID == "" {
		profile, err = a.legacyCallbackPaymentProfile(ctx, provider)
	}
	if err == nil && profile.Provider != provider {
		err = database.ErrConflict
	}
	return profile, profileID, true, err
}

func (a paymentAdapter) legacyCallbackPaymentProfile(ctx context.Context, provider string) (model.PaymentProfileRuntime, error) {
	prefix := "billing." + provider + "."
	profile := model.PaymentProfileRuntime{PaymentProfile: model.PaymentProfile{Provider: provider}}
	var err error
	profile.Endpoint, err = a.settings.Plaintext(ctx, prefix+"base_url")
	if err != nil {
		return model.PaymentProfileRuntime{}, err
	}
	credentialKey := "api_token"
	if provider == "ezpay" {
		credentialKey = "key"
		profile.MerchantID, err = a.settings.Plaintext(ctx, prefix+"merchant_id")
		if err != nil {
			return model.PaymentProfileRuntime{}, err
		}
	}
	profile.CredentialPlaintext, err = a.settings.Plaintext(ctx, prefix+credentialKey)
	return profile, err
}
