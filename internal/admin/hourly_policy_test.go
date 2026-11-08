package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type hourlyPolicyTestRepository struct {
	*adminSettingsRepository
	record database.HourlyPolicyRecord
	saves  int
}

func (r *hourlyPolicyTestRepository) ReadHourlyPolicyRecord(context.Context) (database.HourlyPolicyRecord, error) {
	return r.record, nil
}

func (r *hourlyPolicyTestRepository) CompareAndSwapHourlyPolicy(_ context.Context, _, expected, ciphertext string) (database.HourlyPolicyRecord, error) {
	if r.record.Version != expected {
		return database.HourlyPolicyRecord{}, database.ErrConflict
	}
	r.saves++
	digest := sha256.Sum256([]byte(ciphertext))
	r.record = database.HourlyPolicyRecord{Ciphertext: ciphertext, Version: hex.EncodeToString(digest[:])}
	return r.record, nil
}

func TestHourlyPolicyEncryptedCASAndPausedSalesKeepEnforcementTerms(t *testing.T) {
	ctx := context.Background()
	repo := &hourlyPolicyTestRepository{adminSettingsRepository: newAdminSettingsRepository()}
	settings := NewSettingsService(repo, testVault(t))
	service := NewService(&adminCatalogRepository{}, settings, nil, nil, nil)
	initial, err := service.HourlyPolicy(ctx)
	if err != nil || initial.Version != "" || initial.Policy != model.DefaultHourlyPolicy() {
		t.Fatal("unconfigured policy did not retain disabled defaults", err)
	}
	policy := initial.Policy
	policy.SalesEnabled, policy.ReducedRateBytesPerSecond = true, 125000
	policy.DailyAllowanceBytes, policy.ExtraGBPriceTXBMinor = 3_000_000_000, 123
	first, err := service.SaveHourlyPolicy(ctx, "admin", initial.Version, policy)
	if err != nil || len(first.Version) != 64 || strings.Contains(repo.record.Ciphertext, "salesEnabled") {
		t.Fatal("policy was not encrypted and versioned", err)
	}
	read, err := service.HourlyPolicy(ctx)
	if err != nil || read != first {
		t.Fatal("encrypted policy did not round trip", err)
	}
	policy.SalesEnabled = false
	paused, err := service.SaveHourlyPolicy(ctx, "admin", first.Version, policy)
	if err != nil || paused.Version == first.Version {
		t.Fatal("policy edit did not produce a new ciphertext version", err)
	}
	read, err = settings.HourlyPolicy(ctx)
	if err != nil || read != paused || read.Policy.ReducedRateBytesPerSecond != 125000 || read.Policy.DailyAllowanceBytes != 3_000_000_000 {
		t.Fatal("pausing new sales hid existing enforcement terms", err)
	}
	before := repo.record
	if _, err = service.SaveHourlyPolicy(ctx, "admin", first.Version, policy); !errors.Is(err, database.ErrConflict) || repo.record != before || repo.saves != 2 {
		t.Fatal("stale policy edit changed saved terms", err)
	}
	if err := settings.Put(ctx, "admin", database.HourlyPolicySettingKey, `{}`); err == nil || repo.record != before {
		t.Fatal("generic settings bypass changed hourly terms", err)
	}
}

func TestHourlyPolicyInvalidWritesAndStoredDocumentsFailClosed(t *testing.T) {
	ctx := context.Background()
	repo := &hourlyPolicyTestRepository{adminSettingsRepository: newAdminSettingsRepository()}
	settings := NewSettingsService(repo, testVault(t))
	invalid := model.DefaultHourlyPolicy()
	invalid.ActivationHours = 0
	if _, err := settings.SaveHourlyPolicy(ctx, "admin", "", invalid); !errors.Is(err, model.ErrHourlyPolicyInvalid) || repo.saves != 0 {
		t.Fatal("invalid policy reached encrypted persistence", err)
	}
	defaults, err := json.Marshal(model.DefaultHourlyPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`,
		`{"activationHours":168}`,
		string(defaults) + ` {}`,
		strings.TrimSuffix(string(defaults), "}") + `,"unexpected":true}`,
		`{"activationHours":0,"cooldownSeconds":86400}`,
	} {
		ciphertext, err := settings.vault.Encrypt(database.HourlyPolicySettingKey, raw)
		if err != nil {
			t.Fatal(err)
		}
		repo.record = database.HourlyPolicyRecord{Ciphertext: ciphertext, Version: strings.Repeat("a", 64)}
		if _, err = settings.HourlyPolicy(ctx); !errors.Is(err, ErrHourlyPolicyUnavailable) {
			t.Fatal("invalid saved document received unconfigured defaults", err)
		}
	}
	repo.record.Ciphertext = "invalid-ciphertext"
	if _, err := settings.HourlyPolicy(ctx); !errors.Is(err, ErrHourlyPolicyUnavailable) {
		t.Fatal("unreadable policy did not fail closed", err)
	}
}
