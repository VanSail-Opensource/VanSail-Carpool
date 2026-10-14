package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type paymentProfileRepositoryStub struct {
	profiles  []model.PaymentProfile
	record    database.PaymentProfileRecord
	listErr   error
	recordErr error
	saved     database.PaymentProfileInput
}

func (r *paymentProfileRepositoryStub) ListPaymentProfiles(context.Context) ([]model.PaymentProfile, error) {
	return r.profiles, r.listErr
}

func (r *paymentProfileRepositoryStub) SavePaymentProfile(_ context.Context, input database.PaymentProfileInput) (model.PaymentProfile, error) {
	r.saved = input
	return model.PaymentProfile{ID: input.ID, Provider: input.Provider, ProviderName: input.ProviderName}, nil
}

func (r *paymentProfileRepositoryStub) PaymentProfileRecord(context.Context, string, string) (database.PaymentProfileRecord, error) {
	return r.record, r.recordErr
}

func TestPaymentProfilesReadinessAndListing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	service := NewSettingsService(newAdminSettingsRepository(), testVault(t))

	if profiles, err := service.PaymentProfiles(ctx); err != nil || len(profiles) != 0 {
		t.Fatalf("PaymentProfiles without repository = (%v, %v)", profiles, err)
	}

	repository := &paymentProfileRepositoryStub{profiles: []model.PaymentProfile{{Provider: "ezpay", Enabled: true, Configured: false}}}
	service.SetPaymentProfileRepository(repository)
	issues, found := service.paymentProfileReadiness(ctx, "ezpay")
	if !found || len(issues) != 2 {
		t.Fatalf("paymentProfileReadiness() = (%v, %v)", issues, found)
	}
	if issues, found = service.paymentProfileReadiness(ctx, "bepusdt"); found || issues != nil {
		t.Fatalf("paymentProfileReadiness(missing) = (%v, %v)", issues, found)
	}
	if _, err := service.PaymentProfiles(ctx); err != nil {
		t.Fatalf("PaymentProfiles(): %v", err)
	}

	repository.profiles = []model.PaymentProfile{{Provider: "ezpay", Enabled: false}}
	if issues, found = service.paymentProfileReadiness(ctx, "ezpay"); !found || issues != nil {
		t.Fatalf("disabled readiness = (%v, %v)", issues, found)
	}
	repository.listErr = errors.New("list failed")
	if _, found = service.paymentProfileReadiness(ctx, "ezpay"); found {
		t.Fatal("readiness reported a profile after list failure")
	}
}

func TestSavePaymentProfileNormalizesAndPreservesMaskedCredential(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := &paymentProfileRepositoryStub{}
	service := NewSettingsService(newAdminSettingsRepository(), testVault(t))
	service.SetPaymentProfileRepository(repository)

	input := model.PaymentProfile{ID: " ", Provider: "ezpay", ProviderName: " Pay ", EnabledChannels: []string{" WeChat "}, Endpoint: " https://pay.example ", MerchantID: " merchant ", Credential: " ******** ", Acknowledgement: " ack ", Enabled: true}
	if _, err := service.SavePaymentProfile(ctx, "admin", input); err != nil {
		t.Fatalf("SavePaymentProfile(): %v", err)
	}
	if repository.saved.ID != "ezpay" || repository.saved.ProviderName != "Pay" || repository.saved.Endpoint != "https://pay.example" || repository.saved.CredentialCiphertext != "" || len(repository.saved.EnabledChannels) != 1 || repository.saved.EnabledChannels[0] != "wechat" {
		t.Fatalf("normalized payment profile = %+v", repository.saved)
	}

	cases := []model.PaymentProfile{
		{Provider: "unknown", ProviderName: "x", Endpoint: "https://x.example", EnabledChannels: []string{"x"}},
		{Provider: "ezpay", ProviderName: "", Endpoint: "https://x.example", EnabledChannels: []string{"wechat"}},
		{Provider: "ezpay", ProviderName: "x", Endpoint: "http://x.example", EnabledChannels: []string{"wechat"}},
	}
	for _, invalid := range cases {
		if _, err := service.SavePaymentProfile(ctx, "admin", invalid); err == nil {
			t.Fatalf("SavePaymentProfile(%+v) unexpectedly succeeded", invalid)
		}
	}
}

func TestPaymentProfileRuntimeDecryptsAndFiltersChannels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	settingsRepository := newAdminSettingsRepository()
	service := NewSettingsService(settingsRepository, testVault(t))
	repository := &paymentProfileRepositoryStub{record: database.PaymentProfileRecord{PaymentProfile: model.PaymentProfile{ID: "p1", Provider: "ezpay", EnabledChannels: []string{"wechat"}}, CredentialCiphertext: "bad"}}
	service.SetPaymentProfileRepository(repository)

	if _, err := service.PaymentProfile(ctx, "ezpay", "alipay"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("disabled channel error = %v", err)
	}
	settingsRepository.settings["billing.ezpay.key"] = database.Setting{Key: "billing.ezpay.key", Value: "legacy-key"}
	runtime, err := service.PaymentProfile(ctx, "ezpay", "wechat")
	if err != nil || runtime.CredentialPlaintext != "legacy-key" {
		t.Fatalf("PaymentProfile() = (%+v, %v)", runtime, err)
	}
	if _, err := service.PaymentProfile(ctx, "ezpay", ""); err != nil {
		t.Fatalf("PaymentProfile(any rail): %v", err)
	}
}
