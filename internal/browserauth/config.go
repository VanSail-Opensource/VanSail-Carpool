package browserauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
)

const configKey = "auth.browser.telegram"

var configMu sync.Mutex

// Config is an encrypted runtime OAuth configuration; the secret is never returned to clients.
type Config struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// Config reads the dedicated encrypted runtime setting.
func (s *Service) Config(ctx context.Context) (Config, error) {
	setting, err := s.store.GetSetting(ctx, configKey)
	if errors.Is(err, database.ErrNotFound) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	if !setting.Encrypted {
		return Config{}, errors.New("browser OAuth setting must be encrypted")
	}
	raw, err := s.vault.Decrypt(configKey, setting.Value)
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = json.Unmarshal([]byte(raw), &config)
	return config, err
}

// SaveConfig validates a complete configuration and preserves an existing write-only secret when omitted.
func (s *Service) SaveConfig(ctx context.Context, actorID string, config Config) (Config, error) {
	configMu.Lock()
	defer configMu.Unlock()
	old, err := s.Config(ctx)
	if err != nil {
		return Config{}, err
	}
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	if config.ClientSecret == "" {
		config.ClientSecret = old.ClientSecret
	}
	if config.ClientID != "" {
		id, err := strconv.ParseInt(config.ClientID, 10, 64)
		if err != nil || id <= 0 {
			return Config{}, errors.New("client ID must be a positive Telegram bot ID")
		}
	}
	if len(config.ClientSecret) > 2048 || strings.ContainsAny(config.ClientSecret, "\r\n") {
		return Config{}, errors.New("invalid OAuth client secret")
	}
	if config.Enabled && (config.ClientID == "" || config.ClientSecret == "") {
		return Config{}, errors.New("OAuth credentials are required before enabling")
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return Config{}, err
	}
	encoded, err := s.vault.Encrypt(configKey, string(raw))
	if err != nil {
		return Config{}, err
	}
	if err = s.store.PutSetting(ctx, configKey, encoded, true, &actorID); err != nil {
		return Config{}, err
	}
	return config, nil
}

// CallbackURL is the exact public callback to register with BotFather.
func (s *Service) CallbackURL() string {
	return s.publicURL.ResolveReference(&url.URL{Path: "/api/v1/auth/browser/telegram/callback"}).String()
}
