package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type paymentProfileTestStore struct {
	settings map[string]database.Setting
	profiles []database.PaymentProfileRecord
	orders   map[string]model.PaymentOrder
	orderErr error
}

func (s *paymentProfileTestStore) GetSetting(_ context.Context, key string) (database.Setting, error) {
	if value, ok := s.settings[key]; ok {
		return value, nil
	}
	return database.Setting{}, database.ErrNotFound
}

func (s *paymentProfileTestStore) PutAdminSetting(context.Context, string, string, bool, string, time.Time) error {
	return errors.New("unexpected settings write")
}

func (s *paymentProfileTestStore) ListSettings(context.Context) ([]database.Setting, error) {
	return nil, nil
}

func (s *paymentProfileTestStore) ListPaymentProfiles(context.Context) ([]model.PaymentProfile, error) {
	profiles := make([]model.PaymentProfile, 0, len(s.profiles))
	for _, record := range s.profiles {
		profiles = append(profiles, record.PaymentProfile)
	}
	return profiles, nil
}

func (s *paymentProfileTestStore) SavePaymentProfile(context.Context, database.PaymentProfileInput) (model.PaymentProfile, error) {
	return model.PaymentProfile{}, errors.New("unexpected profile write")
}

func (s *paymentProfileTestStore) PaymentProfileRecord(_ context.Context, provider, _ string) (database.PaymentProfileRecord, error) {
	for _, record := range s.profiles {
		if record.Provider == provider {
			return record, nil
		}
	}
	return database.PaymentProfileRecord{}, database.ErrNotFound
}

func (s *paymentProfileTestStore) PaymentProfileRecordByID(_ context.Context, id, _ string) (database.PaymentProfileRecord, error) {
	for _, record := range s.profiles {
		if record.ID == id {
			return record, nil
		}
	}
	return database.PaymentProfileRecord{}, database.ErrNotFound
}

func (s *paymentProfileTestStore) UserByID(context.Context, string) (model.User, error) {
	return model.User{}, database.ErrNotFound
}

func (s *paymentProfileTestStore) PaymentOrderByID(_ context.Context, id string) (model.PaymentOrder, error) {
	if s.orderErr != nil {
		return model.PaymentOrder{}, s.orderErr
	}
	if order, ok := s.orders[id]; ok {
		return order, nil
	}
	return model.PaymentOrder{}, database.ErrNotFound
}

func paymentProfileAdapterForTest(t *testing.T, provider string) (paymentAdapter, *paymentProfileTestStore) {
	t.Helper()
	vault, err := secret.NewVault([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	store := &paymentProfileTestStore{settings: make(map[string]database.Setting), orders: make(map[string]model.PaymentOrder)}
	for _, item := range []struct{ id, key string }{{"first", "shared-key"}, {"second", "shared-key"}, {provider, "legacy-key"}} {
		ciphertext, err := vault.Encrypt("payment-profile:"+item.id, item.key)
		if err != nil {
			t.Fatal(err)
		}
		store.profiles = append(store.profiles, database.PaymentProfileRecord{PaymentProfile: model.PaymentProfile{
			ID: item.id, Provider: provider, ProviderName: item.id, Endpoint: "https://pay.example", MerchantID: "merchant",
			EnabledChannels: []string{"alipay", "usdt.trc20"}, Configured: true,
		}, CredentialCiphertext: ciphertext})
	}
	for key, value := range map[string]string{"base_url": "https://old.example", "merchant_id": "merchant", "key": "global-key", "api_token": "global-key"} {
		name := "billing." + provider + "." + key
		store.settings[name] = database.Setting{Key: name, Value: value}
	}
	settings := admin.NewSettingsService(store, vault)
	settings.SetPaymentProfileRepository(store)
	return paymentAdapter{settings: settings, users: store}, store
}
