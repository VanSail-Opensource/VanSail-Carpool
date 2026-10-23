package connectivity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// SettingKey stores one atomic configuration in the existing settings registry.
const SettingKey = "connectivity.config"

// DefaultConfig leaves scheduling disabled until an existing account is chosen.
func DefaultConfig() Config {
	return Config{IntervalSeconds: 300, TimeoutSeconds: 15, MaxRetries: 10,
		RetryIntervalSeconds: 1, ProbeURL: "https://cp.cloudflare.com/generate_204"}
}

// DecodeConfig applies defaults and rejects unknown or invalid configuration.
func DecodeConfig(value string) (Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(value) == "" {
		return cfg, nil
	}
	if len(value) > 4096 || strings.TrimSpace(value) == "null" {
		return Config{}, &CodeError{Code: "CONNECTIVITY_INVALID_CONFIG"}
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, &CodeError{Code: "CONNECTIVITY_INVALID_CONFIG", Err: err}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Config{}, &CodeError{Code: "CONNECTIVITY_INVALID_CONFIG"}
	}
	cfg.ProbeURL = strings.TrimSpace(cfg.ProbeURL)
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateConfig(cfg Config) error {
	if cfg.ProxyUserID < 0 || (cfg.ScheduledEnabled && cfg.ProxyUserID == 0) ||
		cfg.IntervalSeconds < 60 || cfg.IntervalSeconds > 86400 || cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds > 60 ||
		cfg.MaxRetries < 0 || cfg.MaxRetries > 10 || cfg.RetryIntervalSeconds < 1 || cfg.RetryIntervalSeconds > 60 {
		return &CodeError{Code: "CONNECTIVITY_INVALID_CONFIG"}
	}
	parsed, err := url.Parse(cfg.ProbeURL)
	if err != nil || len(cfg.ProbeURL) > 2048 || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return &CodeError{Code: "CONNECTIVITY_INVALID_PROBE_URL"}
	}
	return nil
}

// ConfigHash identifies the exact settings under which retained attempts ran.
func ConfigHash(cfg Config) string {
	value := fmt.Sprintf("%t\n%d\n%d\n%d\n%d\n%d\n%s", cfg.ScheduledEnabled, cfg.ProxyUserID,
		cfg.IntervalSeconds, cfg.TimeoutSeconds, cfg.MaxRetries, cfg.RetryIntervalSeconds, cfg.ProbeURL)
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// hostCheckBudget bounds all queued attempts and retry waits for one host.
func (cfg Config) hostCheckBudget() time.Duration {
	seconds := (1+cfg.MaxRetries)*cfg.TimeoutSeconds + cfg.MaxRetries*cfg.RetryIntervalSeconds
	return time.Duration(seconds) * time.Second
}
