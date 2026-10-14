package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestAdminDeductBalance(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		amount int64
		reason string
	}{
		{name: "zero amount", reason: "reason"},
		{name: "negative amount", amount: -1, reason: "reason"},
		{name: "amount too large", amount: 1_000_000_000_001, reason: "reason"},
		{name: "blank reason", amount: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
			if _, err := service.DeductBalance(context.Background(), "admin", "user", test.amount, test.reason); err == nil {
				t.Fatal("DeductBalance() unexpectedly succeeded")
			}
		})
	}

	repository := &adminCatalogRepository{adjustedEntry: model.LedgerEntry{ID: "ledger-1"}}
	service := newAdminServiceForTest(repository, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
	entry, err := service.DeductBalance(context.Background(), "admin", "user", 250, "renewal")
	if err != nil || entry.ID != "ledger-1" || repository.adjustDelta != -250 || repository.adjustReference == "" {
		t.Fatalf("DeductBalance() = (%+v, %v), repository = %+v", entry, err, repository)
	}
	if len(repository.audits) != 1 || repository.audits[0].action != "telegram.balance_deduct" {
		t.Fatalf("DeductBalance() audit = %+v", repository.audits)
	}

	repository.adjustErr = errors.New("deduct failure")
	if _, err := service.DeductBalance(context.Background(), "admin", "user", 1, "retry"); err == nil {
		t.Fatal("DeductBalance() ignored repository failure")
	}
	repository.adjustErr = nil
	repository.auditErr = errors.New("audit failure")
	if _, err := service.DeductBalance(context.Background(), "admin", "user", 1, "retry"); err == nil {
		t.Fatal("DeductBalance() ignored audit failure")
	}
}

type backupDeleterStub struct {
	adminBackupRunner
	id        string
	actorID   string
	deleteErr error
}

func (b *backupDeleterStub) Delete(_ context.Context, id, actorID string) error {
	b.id, b.actorID = id, actorID
	return b.deleteErr
}

func TestAdminDeleteBackup(t *testing.T) {
	t.Parallel()

	service := newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, &adminBackupRunner{}, &adminRefunder{})
	if err := service.DeleteBackup(context.Background(), "admin", "backup-1"); err == nil {
		t.Fatal("DeleteBackup() succeeded without a backup deleter")
	}

	backup := &backupDeleterStub{}
	service = newAdminServiceForTest(&adminCatalogRepository{}, nil, &adminSquadImporter{}, backup, &adminRefunder{})
	if err := service.DeleteBackup(context.Background(), "admin", "backup-1"); err != nil || backup.id != "backup-1" || backup.actorID != "admin" {
		t.Fatalf("DeleteBackup() = %v, backup = %+v", err, backup)
	}
	backup.deleteErr = errors.New("delete failure")
	if err := service.DeleteBackup(context.Background(), "admin", "backup-2"); !errors.Is(err, backup.deleteErr) {
		t.Fatalf("DeleteBackup(error) = %v", err)
	}
}

type paymentProfileByIDRepositoryStub struct {
	*paymentProfileRepositoryStub
	byIDRecord database.PaymentProfileRecord
	byIDErr    error
}

func (r *paymentProfileByIDRepositoryStub) PaymentProfileRecordByID(context.Context, string, string) (database.PaymentProfileRecord, error) {
	return r.byIDRecord, r.byIDErr
}

func TestPaymentProfileByIDAndRuntimes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	if _, err := NewSettingsService(newAdminSettingsRepository(), testVault(t)).PaymentProfileByID(ctx, "missing", ""); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("PaymentProfileByID(without repository) = %v, want ErrNotFound", err)
	}

	settingsRepository := newAdminSettingsRepository()
	settingsRepository.settings["billing.ezpay.key"] = database.Setting{Key: "billing.ezpay.key", Value: "legacy-secret"}
	directRepository := &paymentProfileByIDRepositoryStub{
		paymentProfileRepositoryStub: &paymentProfileRepositoryStub{},
		byIDRecord: database.PaymentProfileRecord{
			PaymentProfile:       model.PaymentProfile{ID: "profile-1", Provider: "ezpay", EnabledChannels: []string{"alipay"}},
			CredentialCiphertext: "invalid-ciphertext",
		},
	}
	service := NewSettingsService(settingsRepository, testVault(t))
	service.SetPaymentProfileRepository(directRepository)
	runtime, err := service.PaymentProfileByID(ctx, "profile-1", "alipay")
	if err != nil || runtime.ID != "profile-1" || runtime.CredentialPlaintext != "legacy-secret" {
		t.Fatalf("PaymentProfileByID() = (%+v, %v)", runtime, err)
	}
	if _, err := service.PaymentProfileByID(ctx, "profile-1", "wxpay"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("PaymentProfileByID(disabled rail) = %v, want ErrNotFound", err)
	}
	directRepository.byIDErr = errors.New("profile lookup failure")
	if _, err := service.PaymentProfileByID(ctx, "profile-1", ""); !errors.Is(err, directRepository.byIDErr) {
		t.Fatalf("PaymentProfileByID() error = %v", err)
	}

	fallbackRepository := &paymentProfileRepositoryStub{
		profiles: []model.PaymentProfile{{ID: "fallback", Provider: "ezpay"}},
		record: database.PaymentProfileRecord{
			PaymentProfile:       model.PaymentProfile{ID: "fallback", Provider: "ezpay", EnabledChannels: []string{"alipay"}},
			CredentialCiphertext: "invalid-ciphertext",
		},
	}
	service.SetPaymentProfileRepository(fallbackRepository)
	if runtime, err := service.PaymentProfileByID(ctx, "fallback", "alipay"); err != nil || runtime.CredentialPlaintext != "legacy-secret" {
		t.Fatalf("PaymentProfileByID(fallback) = (%+v, %v)", runtime, err)
	}
	if _, err := service.PaymentProfileByID(ctx, "unknown", ""); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("PaymentProfileByID(unknown) = %v, want ErrNotFound", err)
	}
	fallbackRepository.listErr = errors.New("profile list failure")
	if _, err := service.PaymentProfileByID(ctx, "fallback", ""); !errors.Is(err, fallbackRepository.listErr) {
		t.Fatalf("PaymentProfileByID(list failure) = %v", err)
	}
	fallbackRepository.listErr = nil
	fallbackRepository.recordErr = errors.New("profile record failure")
	if _, err := service.PaymentProfileByID(ctx, "fallback", ""); !errors.Is(err, fallbackRepository.recordErr) {
		t.Fatalf("PaymentProfileByID(record failure) = %v", err)
	}

	runtimesRepository := &paymentProfileByIDRepositoryStub{
		paymentProfileRepositoryStub: &paymentProfileRepositoryStub{profiles: []model.PaymentProfile{
			{ID: "configured", Provider: "ezpay", Configured: true},
			{ID: "unconfigured", Provider: "ezpay"},
			{ID: "other-provider", Provider: "bepusdt", Configured: true},
		}},
		byIDRecord: database.PaymentProfileRecord{
			PaymentProfile:       model.PaymentProfile{ID: "configured", Provider: "ezpay"},
			CredentialCiphertext: "invalid-ciphertext",
		},
	}
	service.SetPaymentProfileRepository(runtimesRepository)
	runtimes, err := service.PaymentProfileRuntimes(ctx, "ezpay")
	if err != nil || len(runtimes) != 1 || runtimes[0].ID != "configured" {
		t.Fatalf("PaymentProfileRuntimes() = (%+v, %v)", runtimes, err)
	}
	runtimesRepository.byIDErr = errors.New("runtime lookup failure")
	if runtimes, err := service.PaymentProfileRuntimes(ctx, "ezpay"); err != nil || len(runtimes) != 0 {
		t.Fatalf("PaymentProfileRuntimes(runtime failure) = (%+v, %v)", runtimes, err)
	}
	runtimesRepository.listErr = errors.New("runtime list failure")
	if _, err := service.PaymentProfileRuntimes(ctx, "ezpay"); !errors.Is(err, runtimesRepository.listErr) {
		t.Fatalf("PaymentProfileRuntimes(list failure) = %v", err)
	}
}

func TestLegacyEZPayTypeValidator(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"alipay", "wxpay,bank"} {
		if err := validateEZPayType(value); err != nil {
			t.Errorf("validateEZPayType(%q): %v", value, err)
		}
	}
	if err := validateEZPayType("unknown"); err == nil {
		t.Fatal("validateEZPayType() accepted an unknown rail")
	}
}

func TestValidateActivityRewardRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		value   string
		stored  map[string]string
		getErr  error
		wantErr bool
	}{
		{name: "invalid proposed value", key: "activity.daily_reward_min_txb", value: "bad", wantErr: true},
		{name: "missing other boundary", key: "activity.daily_reward_min_txb", value: "1"},
		{name: "repository failure", key: "activity.daily_reward_min_txb", value: "1", getErr: errors.New("settings unavailable"), wantErr: true},
		{name: "invalid stored boundary", key: "activity.daily_reward_min_txb", value: "1", stored: map[string]string{"activity.daily_reward_max_txb": "bad"}, wantErr: true},
		{name: "minimum exceeds maximum", key: "activity.daily_reward_min_txb", value: "2", stored: map[string]string{"activity.daily_reward_max_txb": "1"}, wantErr: true},
		{name: "maximum below minimum", key: "activity.daily_reward_max_txb", value: "1", stored: map[string]string{"activity.daily_reward_min_txb": "2"}, wantErr: true},
		{name: "valid minimum", key: "activity.daily_reward_min_txb", value: "1", stored: map[string]string{"activity.daily_reward_max_txb": "2"}},
		{name: "valid maximum", key: "activity.daily_reward_max_txb", value: "2", stored: map[string]string{"activity.daily_reward_min_txb": "1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newAdminSettingsRepository()
			repository.getErr = test.getErr
			for key, value := range test.stored {
				repository.settings[key] = database.Setting{Key: key, Value: value}
			}
			service := NewSettingsService(repository, testVault(t))
			err := service.validateActivityRewardRange(context.Background(), test.key, test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateActivityRewardRange() error = %v, want error %t", err, test.wantErr)
			}
		})
	}
}
