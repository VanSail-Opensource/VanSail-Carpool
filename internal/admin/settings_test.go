package admin

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func TestSettingsServiceStoresAndDecryptsSecrets(t *testing.T) {
	t.Parallel()

	repository := newAdminSettingsRepository()
	service := NewSettingsService(repository, testVault(t))
	ctx := context.Background()

	if err := service.Put(ctx, "admin-1", "telegram.group_chat_id", "  -1001  "); err != nil {
		t.Fatalf("Put(plain): %v", err)
	}
	plain := repository.settings["telegram.group_chat_id"]
	if plain.Value != "-1001" || plain.Encrypted || repository.lastActor == nil || *repository.lastActor != "admin-1" {
		t.Fatalf("plain stored setting = %+v, actor %v", plain, repository.lastActor)
	}

	if err := service.Put(ctx, "admin-1", "remnawave.api_token", "  bearer-secret  "); err != nil {
		t.Fatalf("Put(secret): %v", err)
	}
	encrypted := repository.settings["remnawave.api_token"]
	if !encrypted.Encrypted || encrypted.Value == "bearer-secret" || !strings.HasPrefix(encrypted.Value, "v1:") {
		t.Fatalf("secret stored in unexpected form: %+v", encrypted)
	}
	decrypted, err := service.Plaintext(ctx, "remnawave.api_token")
	if err != nil || decrypted != "bearer-secret" {
		t.Fatalf("Plaintext(secret) = (%q, %v)", decrypted, err)
	}
	if err := service.Put(ctx, "admin-1", "remnawave.api_token", "  replacement-token  "); err != nil {
		t.Fatalf("Put(secret replacement): %v", err)
	}
	if repository.settings["remnawave.api_token"].Value == encrypted.Value {
		t.Fatal("secret replacement did not update the encrypted database value")
	}
	decrypted, err = service.Plaintext(ctx, "remnawave.api_token")
	if err != nil || decrypted != "replacement-token" {
		t.Fatalf("Plaintext(replaced secret) = (%q, %v)", decrypted, err)
	}
	plainValue, err := service.Plaintext(ctx, "telegram.group_chat_id")
	if err != nil || plainValue != "-1001" {
		t.Fatalf("Plaintext(plain) = (%q, %v)", plainValue, err)
	}

	putCalls := repository.putCalls
	if err := service.Put(ctx, "admin-1", "remnawave.api_token", ""); err != nil {
		t.Fatalf("Put(empty existing secret): %v", err)
	}
	if repository.putCalls != putCalls {
		t.Fatal("empty existing secret overwrote the stored credential")
	}
}

func TestSettingsServiceErrorsAndOptional(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := newAdminSettingsRepository()
	service := NewSettingsService(repository, testVault(t))

	if err := service.Put(ctx, "admin", "unknown.setting", "value"); err == nil {
		t.Fatal("unknown setting unexpectedly accepted")
	}
	if err := service.Put(ctx, "admin", "remnawave.api_token", ""); err == nil {
		t.Fatal("empty required secret unexpectedly accepted")
	}
	optional, err := service.Optional(ctx, "billing.ezpay.enabled")
	if err != nil || optional != "" {
		t.Fatalf("Optional(missing) = (%q, %v)", optional, err)
	}
	repository.getErr = errors.New("storage unavailable")
	if _, err := service.Optional(ctx, "billing.ezpay.enabled"); err == nil {
		t.Fatal("Optional() ignored repository error")
	}

	repository.getErr = nil
	repository.settings["remnawave.api_token"] = database.Setting{Key: "remnawave.api_token", Value: "v1:invalid", Encrypted: true}
	if _, err := service.Plaintext(ctx, "remnawave.api_token"); err == nil {
		t.Fatal("Plaintext() accepted invalid ciphertext")
	}
	repository.putErr = errors.New("write failure")
	if err := service.Put(ctx, "admin", "telegram.group_chat_id", "-1001"); err == nil {
		t.Fatal("Put() ignored repository error")
	}
}

func TestSettingsSafeListMasksAndSorts(t *testing.T) {
	t.Parallel()

	updated := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	repository := newAdminSettingsRepository()
	repository.settings["telegram.group_chat_id"] = database.Setting{Key: "telegram.group_chat_id", Value: "-1001", UpdatedAt: updated}
	repository.settings["remnawave.api_token"] = database.Setting{Key: "remnawave.api_token", Value: "ciphertext", Encrypted: true, UpdatedAt: updated}
	repository.settings["not.registered"] = database.Setting{Key: "not.registered", Value: "ignored", UpdatedAt: updated}
	service := NewSettingsService(repository, testVault(t))

	settings, err := service.SafeList(context.Background())
	if err != nil {
		t.Fatalf("SafeList(): %v", err)
	}
	if len(settings) != len(settingDefinitions) {
		t.Fatalf("SafeList() length = %d, want %d", len(settings), len(settingDefinitions))
	}
	keys := make([]string, len(settings))
	for index, setting := range settings {
		keys[index] = setting.Key
		if setting.Key == "remnawave.api_token" && (setting.Value != "" || !setting.Encrypted || !setting.Configured) {
			t.Fatalf("secret SafeList entry = %+v", setting)
		}
		if setting.Key == "telegram.group_chat_id" && (setting.Value != "-1001" || setting.Encrypted || !setting.Configured) {
			t.Fatalf("plain SafeList entry = %+v", setting)
		}
	}
	if !sort.StringsAreSorted(keys) {
		t.Fatalf("SafeList keys are not sorted: %v", keys)
	}

	repository.listErr = errors.New("list failure")
	if _, err := service.SafeList(context.Background()); err == nil {
		t.Fatal("SafeList() ignored repository error")
	}
}

func TestSettingsReadiness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := newAdminSettingsRepository()
	service := NewSettingsService(repository, testVault(t))
	issues := service.Readiness(ctx, 0)
	if !containsString(issues, "missing:catalog.active_combo") || !containsString(issues, "missing_or_invalid:remnawave.api_token") {
		t.Fatalf("Readiness(empty) = %v", issues)
	}

	required := map[string]string{
		"telegram.group_chat_id":   "-1001",
		"telegram.channel_chat_id": "-1002",
		"remnawave.base_url":       "http://remnawave:3000",
		"remnawave.api_token":      "secret",
		"billing.rate.txb_per_cny": "1",
		"billing.rate.txb_per_usd": "10",
		"billing.rate.txb_per_xtr": "0.5",
	}
	for key, value := range required {
		if err := service.Put(ctx, "admin", key, value); err != nil {
			t.Fatalf("Put(%s): %v", key, err)
		}
	}
	if issues := service.Readiness(ctx, 1); len(issues) != 0 {
		t.Fatalf("Readiness(configured) = %v", issues)
	}

	if err := service.Put(ctx, "admin", "billing.ezpay.enabled", "true"); err != nil {
		t.Fatalf("enable EZPay: %v", err)
	}
	if err := service.Put(ctx, "admin", "billing.bepusdt.enabled", "true"); err != nil {
		t.Fatalf("enable BEPusdt: %v", err)
	}
	issues = service.Readiness(ctx, 1)
	for _, want := range []string{
		"missing:billing.ezpay.base_url", "missing:billing.ezpay.merchant_id", "missing:billing.ezpay.key", "missing:billing.ezpay.methods",
		"missing:billing.bepusdt.base_url", "missing:billing.bepusdt.api_token", "missing:billing.bepusdt.methods",
	} {
		if !containsString(issues, want) {
			t.Fatalf("Readiness(enabled providers) = %v, missing %q", issues, want)
		}
	}
}

func TestSettingValidators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func(string) error
		valid    []string
		invalid  []string
	}{
		{name: "positive decimal", validate: validatePositiveDecimal, valid: []string{"0.01", "42"}, invalid: []string{"0", "-1", "one"}},
		{name: "integer", validate: validateInteger, valid: []string{"1", "-1001"}, invalid: []string{"0", "1.5", "x"}},
		{name: "optional integer", validate: validateOptionalInteger, valid: []string{"", "-1001"}, invalid: []string{"0", "1.5", "x"}},
		{name: "HTTPS URL", validate: validateHTTPSURL, valid: []string{"https://example.test/path"}, invalid: []string{"http://example.test", "https://user@example.test", "/relative", "https:///missing"}},
		{name: "HTTP or HTTPS URL", validate: validateHTTPOrHTTPSURL, valid: []string{"http://remnawave:3000", "https://example.test/path"}, invalid: []string{"https://user@example.test", "/relative", "http:///missing"}},
		{name: "boolean", validate: validateBoolean, valid: []string{"true", "false"}, invalid: []string{"TRUE", "1", ""}},
		{name: "ack", validate: validateAck, valid: []string{"ok", "success", "SUCCESS"}, invalid: []string{"true", "done"}},
		{name: "EZPay methods", validate: billing.ValidateEZPayMethods, valid: []string{"alipay", "wxpay,alipay", "qqpay,bank,jdpay"}, invalid: []string{"card", "", "alipay,alipay"}},
		{name: "nonempty", validate: nonempty, valid: []string{"value"}, invalid: []string{""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, value := range test.valid {
				if err := test.validate(value); err != nil {
					t.Errorf("validate(%q): %v", value, err)
				}
			}
			for _, value := range test.invalid {
				if err := test.validate(value); err == nil {
					t.Errorf("validate(%q) unexpectedly succeeded", value)
				}
			}
		})
	}
}

type adminSettingsRepository struct {
	settings  map[string]database.Setting
	putErr    error
	getErr    error
	listErr   error
	putCalls  int
	lastActor *string
}

func newAdminSettingsRepository() *adminSettingsRepository {
	return &adminSettingsRepository{settings: make(map[string]database.Setting)}
}

func (r *adminSettingsRepository) PutSetting(_ context.Context, key, value string, encrypted bool, actorID *string) error {
	if r.putErr != nil {
		return r.putErr
	}
	r.putCalls++
	r.lastActor = actorID
	r.settings[key] = database.Setting{Key: key, Value: value, Encrypted: encrypted, UpdatedAt: time.Now().UTC()}
	return nil
}
func (r *adminSettingsRepository) GetSetting(_ context.Context, key string) (database.Setting, error) {
	if r.getErr != nil {
		return database.Setting{}, r.getErr
	}
	setting, ok := r.settings[key]
	if !ok {
		return database.Setting{}, database.ErrNotFound
	}
	return setting, nil
}
func (r *adminSettingsRepository) ListSettings(context.Context) ([]database.Setting, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	result := make([]database.Setting, 0, len(r.settings))
	for _, setting := range r.settings {
		result = append(result, setting)
	}
	return result, nil
}

func testVault(t *testing.T) *secret.Vault {
	t.Helper()
	vault, err := secret.NewVault([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewVault(): %v", err)
	}
	return vault
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
