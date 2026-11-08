package admin

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

var ErrEmbyConnection = errors.New("invalid Emby connection")
var ErrEmbyFreshToken = errors.New("changing the Emby address requires a fresh token")

type embyConnectionRepository interface {
	EmbyConnection(context.Context) (database.EmbyConnectionRecord, error)
	SaveEmbyConnection(context.Context, string, database.EmbyConnectionRecord, string) error
}

// EmbyConnectionView exposes configuration presence and an opaque concurrency token only.
type EmbyConnectionView struct {
	Version         string `json:"version"`
	URLConfigured   bool   `json:"urlConfigured"`
	TokenConfigured bool   `json:"tokenConfigured"`
}

type EmbyConnectionWrite struct {
	Version  string `json:"version"`
	BaseURL  string `json:"baseURL"`
	APIToken string `json:"apiToken"`
}

func embyConnectionView(record database.EmbyConnectionRecord) EmbyConnectionView {
	return EmbyConnectionView{record.Version(), record.BaseURL.Value != "", record.Token.Value != ""}
}

func (s *SettingsService) embySnapshot(ctx context.Context) (database.EmbyConnectionRecord, error) {
	repository, ok := s.repository.(embyConnectionRepository)
	if !ok {
		return database.EmbyConnectionRecord{}, ErrEmbyConnection
	}
	return repository.EmbyConnection(ctx)
}

func (s *SettingsService) EmbyConnection(ctx context.Context) (EmbyConnectionView, error) {
	record, err := s.embySnapshot(ctx)
	return embyConnectionView(record), err
}

func (s *SettingsService) decryptEmby(record database.EmbyConnectionRecord) (string, string, error) {
	values := make([]string, 2)
	for i, setting := range []database.Setting{record.BaseURL, record.Token} {
		values[i] = setting.Value
		if setting.Encrypted && setting.Value != "" {
			var err error
			values[i], err = s.vault.Decrypt(setting.Key, setting.Value)
			if err != nil {
				return "", "", ErrEmbyConnection
			}
		}
	}
	return values[0], values[1], nil
}

// EmbyCredentials reads one SQL snapshot and validates it before any queued network call.
func (s *SettingsService) EmbyCredentials(ctx context.Context) (string, string, error) {
	record, err := s.embySnapshot(ctx)
	if err != nil {
		return "", "", err
	}
	baseURL, token, err := s.decryptEmby(record)
	if err != nil || validateEmbyCredentials(baseURL, token) != nil {
		return "", "", ErrEmbyConnection
	}
	return baseURL, token, nil
}

func (s *SettingsService) SaveEmbyConnection(ctx context.Context, actor string, input EmbyConnectionWrite) (EmbyConnectionView, error) {
	current, err := s.embySnapshot(ctx)
	if err != nil {
		return EmbyConnectionView{}, err
	}
	if input.Version == "" || input.Version != current.Version() {
		return EmbyConnectionView{}, database.ErrConflict
	}
	baseURL, token, err := s.decryptEmby(current)
	if err != nil {
		return EmbyConnectionView{}, err
	}
	input.BaseURL, input.APIToken = strings.TrimSpace(input.BaseURL), strings.TrimSpace(input.APIToken)
	if input.BaseURL != "" && input.BaseURL != baseURL {
		if input.APIToken == "" || input.APIToken == token {
			return EmbyConnectionView{}, ErrEmbyFreshToken
		}
		baseURL = input.BaseURL
	}
	if input.APIToken != "" {
		token = input.APIToken
	}
	if validateEmbyCredentials(baseURL, token) != nil {
		return EmbyConnectionView{}, ErrEmbyConnection
	}
	for _, entry := range []struct {
		key, value string
		target     *database.Setting
	}{
		{database.EmbyURLSetting, baseURL, &current.BaseURL}, {database.EmbyTokenSetting, token, &current.Token},
	} {
		sealed, err := s.vault.Encrypt(entry.key, entry.value)
		if err != nil {
			return EmbyConnectionView{}, err
		}
		*entry.target = database.Setting{Key: entry.key, Value: sealed, Encrypted: true}
	}
	if err := s.repository.(embyConnectionRepository).SaveEmbyConnection(ctx, input.Version, current, actor); err != nil {
		return EmbyConnectionView{}, err
	}
	return embyConnectionView(current), nil
}

func (s *Service) EmbyConnection(ctx context.Context) (EmbyConnectionView, error) {
	return s.settings.EmbyConnection(ctx)
}

func (s *Service) SaveEmbyConnection(ctx context.Context, actor string, input EmbyConnectionWrite) (EmbyConnectionView, error) {
	view, err := s.settings.SaveEmbyConnection(ctx, actor, input)
	if err != nil {
		return view, err
	}
	err = s.audit(ctx, actor, "emby.connection.update", "setting", "emby.connection", map[string]any{"secret": true})
	return view, err
}

func validateEmbyCredentials(baseURL, token string) error {
	if len(baseURL) > 2048 || validateHTTPSURL(baseURL) != nil || len(token) > 4096 || strings.TrimSpace(token) == "" || strings.ContainsFunc(token, unicode.IsControl) {
		return ErrEmbyConnection
	}
	return nil
}
