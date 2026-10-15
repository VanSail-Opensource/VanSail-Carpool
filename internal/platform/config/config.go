// Package config loads and validates process bootstrap configuration.
package config

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config contains the values required before encrypted dashboard settings can be read.
type Config struct {
	Port                 string
	DataDir              string
	DatabasePath         string
	ProxyCacheMaxBytes   int64
	PublicBaseURL        *url.URL
	AdminTelegramIDs     []int64
	TelegramBotToken     string
	MasterKey            []byte
	Timezone             *time.Location
	LogLevel             slog.Level
	SessionTTL           time.Duration
	InitDataMaxAge       time.Duration
	ShutdownTimeout      time.Duration
	BackupRetention      time.Duration
	BackupUploadMaxBytes int64
	BackupHour           int
	AllowInsecureHTTP    bool
}

// Load reads configuration from the environment and returns an actionable error for invalid input.
func Load() (Config, error) {
	var cfg Config
	cfg.Port = envDefault("PORT", "8080")
	cfg.DataDir = envDefault("DATA_DIR", "/data")
	cfg.DatabasePath = filepath.Join(cfg.DataDir, "tx-carpool.db")
	cfg.ProxyCacheMaxBytes = 1 << 30
	if value := strings.TrimSpace(os.Getenv("PROXY_CACHE_MAX_BYTES")); value != "" {
		maximum, err := strconv.ParseInt(value, 10, 64)
		if err != nil || maximum < 16<<20 || maximum > 16<<30 {
			return Config{}, fmt.Errorf("PROXY_CACHE_MAX_BYTES must be between 16777216 and 17179869184")
		}
		cfg.ProxyCacheMaxBytes = maximum
	}
	cfg.SessionTTL = 7 * 24 * time.Hour
	cfg.InitDataMaxAge = 5 * time.Minute
	cfg.ShutdownTimeout = 15 * time.Second
	cfg.BackupRetention = 7 * 24 * time.Hour
	cfg.BackupUploadMaxBytes = 2 << 30
	cfg.BackupHour = 2
	cfg.AllowInsecureHTTP = strings.EqualFold(os.Getenv("ALLOW_INSECURE_HTTP"), "true")
	if value := strings.TrimSpace(os.Getenv("BACKUP_UPLOAD_MAX_BYTES")); value != "" {
		maximum, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || maximum < 1<<20 || maximum > 16<<30 {
			return Config{}, fmt.Errorf("BACKUP_UPLOAD_MAX_BYTES must be between 1048576 and 17179869184")
		}
		cfg.BackupUploadMaxBytes = maximum
	}

	adminIDs, err := parseAdminTelegramIDs(os.Getenv("ADMIN_TELEGRAM_ID"))
	if err != nil {
		return Config{}, err
	}
	cfg.AdminTelegramIDs = adminIDs

	cfg.TelegramBotToken = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	parsedURL, err := url.Parse(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")))
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || (parsedURL.Path != "" && parsedURL.Path != "/") {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL must be an absolute origin without credentials, path, query, or fragment")
	}
	if parsedURL.Scheme != "https" && (!cfg.AllowInsecureHTTP || parsedURL.Scheme != "http") {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL must use https")
	}
	parsedURL.Path = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	cfg.PublicBaseURL = parsedURL

	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("CONFIG_MASTER_KEY")))
	if err != nil || len(key) != 32 {
		return Config{}, fmt.Errorf("CONFIG_MASTER_KEY must be base64-encoded 32 bytes")
	}
	cfg.MasterKey = key

	timezone := envDefault("TZ", "UTC")
	cfg.Timezone, err = time.LoadLocation(timezone)
	if err != nil {
		return Config{}, fmt.Errorf("load TZ %q: %w", timezone, err)
	}

	switch strings.ToLower(envDefault("LOG_LEVEL", "info")) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}
	return cfg, nil
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func parseAdminTelegramIDs(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, fmt.Errorf("ADMIN_TELEGRAM_ID must contain comma-separated positive integers")
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("ADMIN_TELEGRAM_ID must contain comma-separated positive integers")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}
