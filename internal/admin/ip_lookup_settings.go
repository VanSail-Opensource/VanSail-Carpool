package admin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type ipLookupSettingsRepository interface {
	SaveIPLookupSettings(context.Context, string, iplookup.Config, map[string]string, map[string]*int) error
	ListCombos(context.Context, bool) ([]model.Combo, error)
}

func init() {
	settingDefinitions[iplookup.SettingKey] = SettingDefinition{Validate: func(v string) error { _, err := iplookup.DecodeConfig(v); return err }}
	for _, id := range iplookup.IDs {
		settingDefinitions[iplookup.CredentialKey(id)] = SettingDefinition{Secret: true}
	}
	settingDefinitions[iplookup.CredentialKey(iplookup.CloudflareID)] = SettingDefinition{Secret: true}
}

// IPLookupSettings exposes only safe configuration and credential presence.
func (s *SettingsService) IPLookupSettings(ctx context.Context) (iplookup.AdminSettings, error) {
	result := iplookup.AdminSettings{Credentials: map[string]iplookup.CredentialEdit{}, Configured: map[string]bool{}, ComboQuotas: map[string]*int{}}
	raw, err := s.Optional(ctx, iplookup.SettingKey)
	if err != nil {
		return result, err
	}
	result.Config, err = iplookup.DecodeConfig(raw)
	if err != nil {
		return result, err
	}
	for _, id := range append(append([]string{}, iplookup.IDs...), iplookup.CloudflareID) {
		value, err := s.Optional(ctx, iplookup.CredentialKey(id))
		if err != nil {
			return result, err
		}
		result.Configured[id] = value != ""
	}
	repo, ok := s.repository.(ipLookupSettingsRepository)
	if !ok {
		return result, &iplookup.CodeError{Code: "IP_LOOKUP_UNAVAILABLE"}
	}
	combos, err := repo.ListCombos(ctx, false)
	if err != nil {
		return result, err
	}
	for _, combo := range combos {
		result.ComboQuotas[combo.ID] = combo.IPLookupQuota
	}
	return result, nil
}

// PutIPLookupSettings validates one atomic settings change; blank credentials mean keep.
func (s *SettingsService) PutIPLookupSettings(ctx context.Context, actor string, input iplookup.AdminSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(input.Config)
	if err != nil {
		return err
	}
	c, err := iplookup.DecodeConfig(string(raw))
	if err != nil {
		return err
	}
	secrets := map[string]string{}
	for id, edit := range input.Credentials {
		if _, ok := settingDefinitions[iplookup.CredentialKey(id)]; !ok || (edit.Clear && edit.Value != "") {
			return &iplookup.CodeError{Code: "IP_LOOKUP_INVALID_CONFIG"}
		}
		if edit.Clear {
			secrets[id] = ""
			continue
		}
		value := strings.TrimSpace(edit.Value)
		if value == "" {
			continue
		}
		if len(value) > 512 || strings.ContainsAny(value, "\r\n\x00") {
			return &iplookup.CodeError{Code: "IP_LOOKUP_INVALID_CONFIG"}
		}
		secrets[id], err = s.vault.Encrypt(iplookup.CredentialKey(id), value)
		if err != nil {
			return err
		}
	}
	repo, ok := s.repository.(ipLookupSettingsRepository)
	if !ok {
		return &iplookup.CodeError{Code: "IP_LOOKUP_UNAVAILABLE"}
	}
	return repo.SaveIPLookupSettings(ctx, actor, c, secrets, input.ComboQuotas)
}
