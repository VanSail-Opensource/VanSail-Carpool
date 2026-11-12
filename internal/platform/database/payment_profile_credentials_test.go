package database

import (
	"context"
	"errors"
	"testing"
)

func TestPaymentProfileRetainedCredentialCannotChangeDestination(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"endpoint", "provider", "merchant"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := newTestStore(t)
			input := PaymentProfileInput{ID: "guarded-profile", Provider: "ezpay", ProviderName: "Original",
				EnabledChannels: []string{"alipay"}, Endpoint: "https://original.example", MerchantID: "merchant",
				CredentialCiphertext: "original-ciphertext", Enabled: true}
			if _, err := store.SavePaymentProfile(ctx, input); err != nil {
				t.Fatal(err)
			}
			input.CredentialCiphertext, input.ProviderName = "", "Renamed"
			if _, err := store.SavePaymentProfile(ctx, input); err != nil {
				t.Fatal("cosmetic edit must retain credentials", err)
			}
			switch field {
			case "endpoint":
				input.Endpoint = "https://replacement.example"
			case "provider":
				input.Provider = "bepusdt"
			case "merchant":
				input.MerchantID = "replacement-merchant"
			}
			if _, err := store.SavePaymentProfile(ctx, input); !errors.Is(err, ErrConflict) {
				t.Fatalf("%s changed with retained secret: %v", field, err)
			}
			stored, err := store.PaymentProfileRecordByID(ctx, input.ID, "")
			if err != nil || stored.Endpoint != "https://original.example" || stored.Provider != "ezpay" || stored.MerchantID != "merchant" || stored.CredentialCiphertext != "original-ciphertext" || stored.ProviderName != "Renamed" {
				t.Fatal("rejected update mutated provider identity or credentials", err)
			}
			input.CredentialCiphertext = "replacement-ciphertext"
			if _, err := store.SavePaymentProfile(ctx, input); err != nil {
				t.Fatal("explicit replacement credentials must permit destination change", err)
			}
		})
	}
}
