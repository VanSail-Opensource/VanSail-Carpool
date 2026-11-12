package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestPaymentProfileLegacyCredentialRequiresMigrationEvidence(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"ezpay", "bepusdt"} {
		for _, scenario := range []string{"migrated", "custom-id", "different-provider", "different-ciphertext", "unencrypted-setting", "missing-setting", "corrupt-ciphertext"} {
			t.Run(provider+"/"+scenario, func(t *testing.T) {
				t.Parallel()
				vault := testVault(t)
				key := "billing." + provider + ".api_token"
				if provider == "ezpay" {
					key = "billing.ezpay.key"
				}
				ciphertext, err := vault.Encrypt(key, "migrated-secret")
				if err != nil {
					t.Fatal(err)
				}
				record := database.PaymentProfileRecord{PaymentProfile: model.PaymentProfile{ID: provider, Provider: provider}, CredentialCiphertext: ciphertext}
				setting := database.Setting{Key: key, Value: ciphertext, Encrypted: true}
				switch scenario {
				case "custom-id":
					record.ID = "another-account"
				case "different-provider":
					record.Provider = "other"
				case "different-ciphertext":
					setting.Value, err = vault.Encrypt(key, "replacement-secret")
					if err != nil {
						t.Fatal(err)
					}
				case "unencrypted-setting":
					setting.Encrypted = false
				case "corrupt-ciphertext":
					record.CredentialCiphertext, setting.Value = "broken", "broken"
				}
				settings := newAdminSettingsRepository()
				if scenario != "missing-setting" {
					settings.settings[key] = setting
				}
				repository := &paymentProfileByIDRepositoryStub{paymentProfileRepositoryStub: &paymentProfileRepositoryStub{record: record}, byIDRecord: record}
				service := NewSettingsService(settings, vault)
				service.SetPaymentProfileRepository(repository)
				for _, byID := range []bool{false, true} {
					var runtime model.PaymentProfileRuntime
					if byID {
						runtime, err = service.PaymentProfileByID(context.Background(), record.ID, "")
					} else {
						runtime, err = service.PaymentProfile(context.Background(), record.Provider, "")
					}
					if scenario == "migrated" {
						if err != nil || runtime.CredentialPlaintext != "migrated-secret" {
							t.Fatalf("runtime(byID=%t) = (%+v, %v)", byID, runtime, err)
						}
					} else if err == nil || runtime.CredentialPlaintext != "" {
						t.Fatalf("unproven credential fallback(byID=%t) = (%+v, %v)", byID, runtime, err)
					}
				}
			})
		}
	}
}

func TestPaymentProfileByIDRejectsAnotherAccount(t *testing.T) {
	t.Parallel()
	ciphertext, err := testVault(t).Encrypt("payment-profile:first", "first-secret")
	if err != nil {
		t.Fatal(err)
	}
	repository := &paymentProfileRepositoryStub{
		profiles: []model.PaymentProfile{{ID: "second", Provider: "ezpay"}},
		record:   database.PaymentProfileRecord{PaymentProfile: model.PaymentProfile{ID: "first", Provider: "ezpay"}, CredentialCiphertext: ciphertext},
	}
	service := NewSettingsService(newAdminSettingsRepository(), testVault(t))
	service.SetPaymentProfileRepository(repository)
	if _, err := service.PaymentProfileByID(context.Background(), "second", ""); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("profile mismatch error = %v", err)
	}
}

func TestPaymentProfileRetargetRequiresExplicitCredential(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"endpoint", "provider", "merchant", "name"} {
		for _, credential := range []string{"", " ******** ", "replacement-secret"} {
			t.Run(field+"/"+credential, func(t *testing.T) {
				t.Parallel()
				current := model.PaymentProfile{ID: "account", Provider: "ezpay", ProviderName: "Pay", Endpoint: "https://pay.example", MerchantID: "merchant", EnabledChannels: []string{"alipay"}, Configured: true}
				repository := &paymentProfileRepositoryStub{profiles: []model.PaymentProfile{current}}
				service := NewSettingsService(newAdminSettingsRepository(), testVault(t))
				service.SetPaymentProfileRepository(repository)
				input := current
				input.Credential = credential
				switch field {
				case "endpoint":
					input.Endpoint = "https://replacement.example"
				case "provider":
					input.Provider = "bepusdt"
				case "merchant":
					input.MerchantID = "replacement-merchant"
				case "name":
					input.ProviderName = "New display name"
				}
				_, err := service.SavePaymentProfile(context.Background(), "admin", input)
				if field != "name" && credential != "replacement-secret" {
					if !errors.Is(err, ErrPaymentProfileFreshCredential) || repository.saved.ID != "" {
						t.Fatalf("retarget saved=%+v error=%v", repository.saved, err)
					}
					return
				}
				if err != nil || repository.saved.ID != current.ID {
					t.Fatalf("valid update saved=%+v error=%v", repository.saved, err)
				}
				if credential == "replacement-secret" {
					plain, decryptErr := testVault(t).Decrypt("payment-profile:"+current.ID, repository.saved.CredentialCiphertext)
					if decryptErr != nil || plain != credential {
						t.Fatalf("replacement encryption = %q, %v", plain, decryptErr)
					}
				}
			})
		}
	}
}

func TestMigratedPaymentAccountSettingsCannotBeRewritten(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"billing.ezpay.base_url", "billing.ezpay.merchant_id", "billing.ezpay.key", "billing.bepusdt.base_url", "billing.bepusdt.api_token"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			repository := newAdminSettingsRepository()
			original := database.Setting{Key: key, Value: "immutable-migration-value", Encrypted: true}
			repository.settings[key] = original
			service := NewSettingsService(repository, testVault(t))
			service.SetPaymentProfileRepository(&paymentProfileRepositoryStub{profiles: []model.PaymentProfile{
				{ID: "ezpay", Provider: "ezpay"}, {ID: "bepusdt", Provider: "bepusdt"},
			}})
			if err := service.Put(context.Background(), "admin", key, "https://replacement.example"); !errors.Is(err, ErrPaymentProfileSettings) {
				t.Fatalf("legacy setting write = %v", err)
			}
			if repository.putCalls != 0 || repository.settings[key] != original {
				t.Fatal("legacy migration evidence changed")
			}
		})
	}
}
