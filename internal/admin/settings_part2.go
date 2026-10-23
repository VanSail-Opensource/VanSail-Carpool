package admin

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"sort"
	"strings"
)

func (s *SettingsService) SafeList(ctx context.Context) ([]model.Setting, error) {
	stored, err := s.repository.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]database.Setting, len(stored))
	for _, setting := range stored {
		byKey[setting.Key] = setting
	}
	keys := make([]string, 0, len(settingDefinitions))
	for key := range settingDefinitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]model.Setting, 0, len(keys))
	for _, key := range keys {
		definition := settingDefinitions[key]
		setting, exists := byKey[key]
		value := definition.Default
		if exists {
			value = setting.Value
		}
		if definition.Secret {
			value = ""
		}
		result = append(result, model.Setting{Key: key, Value: value, Encrypted: definition.Secret, Configured: exists && setting.Value != "", Category: settingCategory(key), UpdatedAt: setting.UpdatedAt})
	}
	return result, nil
}

func settingCategory(key string) string {
	switch {
	case strings.HasPrefix(key, "captcha."):
		return "captcha"
	case strings.HasPrefix(key, "telegram."):
		return "telegram"
	case strings.HasPrefix(key, "billing.rate."):
		return "rates"
	case strings.HasPrefix(key, "billing."):
		return "payments"
	case strings.HasPrefix(key, "emby."):
		return "emby"
	case strings.HasPrefix(key, "activity."):
		return "activity"
	case strings.HasPrefix(key, "renewal."):
		return "renewal"
	default:
		return "application"
	}
}

// Readiness returns stable setup issue identifiers for operations and the admin UI.

func (s *SettingsService) Readiness(ctx context.Context, activeComboCount int) []string {
	issues := make([]string, 0)
	for key, definition := range settingDefinitions {
		if !definition.Required {
			continue
		}
		value, err := s.Optional(ctx, key)
		if err != nil || value == "" || (definition.Validate != nil && definition.Validate(value) != nil) {
			issues = append(issues, "missing_or_invalid:"+key)
		}
	}
	for _, provider := range []string{"ezpay", "bepusdt"} {
		if profileIssues, handled := s.paymentProfileReadiness(ctx, provider); handled {
			issues = append(issues, profileIssues...)
			continue
		}
		enabled, _ := s.Optional(ctx, "billing."+provider+".enabled")
		if enabled != "true" {
			continue
		}
		keys := []string{"base_url"}
		if provider == "ezpay" {
			keys = append(keys, "merchant_id", "key", "methods")
		} else {
			keys = append(keys, "api_token", "methods")
		}
		for _, suffix := range keys {
			if value, err := s.Optional(ctx, "billing."+provider+"."+suffix); err != nil || value == "" {
				issues = append(issues, "missing:billing."+provider+"."+suffix)
			}
		}
	}
	if activeComboCount == 0 {
		issues = append(issues, "missing:catalog.active_combo")
	}
	sort.Strings(issues)
	return issues
}
