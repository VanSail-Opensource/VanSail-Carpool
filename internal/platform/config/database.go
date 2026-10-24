package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DatabaseConfig selects one authoritative backend. URL is a secret and must
// never be logged. A missing/unavailable backend never selects another driver.
type DatabaseConfig struct {
	Driver           string
	URL              string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	ConnectTimeout   time.Duration
	StatementTimeout time.Duration
	LockTimeout      time.Duration
	MigrateOnStart   bool
}

func loadDatabase() (DatabaseConfig, error) {
	cfg := DatabaseConfig{Driver: strings.ToLower(envDefault("DB_DRIVER", "sqlite"))}
	if cfg.Driver != "sqlite" && cfg.Driver != "postgres" {
		return cfg, fmt.Errorf("DB_DRIVER must be sqlite or postgres")
	}
	var err error
	cfg.URL, err = databaseSecret()
	if err != nil {
		return cfg, err
	}
	if cfg.Driver == "postgres" {
		parsed, parseErr := url.Parse(cfg.URL)
		if parseErr != nil || parsed == nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || strings.Trim(parsed.Path, "/") == "" || parsed.Fragment != "" {
			return cfg, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL with host and database")
		}
	} else if cfg.URL != "" {
		return cfg, fmt.Errorf("DATABASE_URL requires explicit DB_DRIVER=postgres")
	}
	defaultOpen := 8
	if cfg.Driver == "sqlite" {
		defaultOpen = 4
	}
	cfg.MaxOpenConns, err = databaseInt("DB_MAX_OPEN_CONNS", defaultOpen, 1, 128)
	if err != nil {
		return cfg, err
	}
	cfg.MaxIdleConns, err = databaseInt("DB_MAX_IDLE_CONNS", min(4, cfg.MaxOpenConns), 0, cfg.MaxOpenConns)
	if err != nil {
		return cfg, err
	}
	for _, item := range []struct {
		name     string
		fallback time.Duration
		target   *time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", 30 * time.Minute, &cfg.ConnMaxLifetime},
		{"DB_CONNECT_TIMEOUT", 10 * time.Second, &cfg.ConnectTimeout},
		{"DB_STATEMENT_TIMEOUT", 30 * time.Second, &cfg.StatementTimeout},
		{"DB_LOCK_TIMEOUT", 5 * time.Second, &cfg.LockTimeout},
	} {
		value, e := time.ParseDuration(envDefault(item.name, item.fallback.String()))
		if e != nil || value < time.Millisecond || value > 24*time.Hour {
			return cfg, fmt.Errorf("%s must be a duration between 1ms and 24h", item.name)
		}
		*item.target = value
	}
	if cfg.LockTimeout > cfg.StatementTimeout {
		return cfg, fmt.Errorf("DB_LOCK_TIMEOUT must not exceed DB_STATEMENT_TIMEOUT")
	}
	// Preserve historical SQLite startup behavior. PostgreSQL migration is opt-in.
	cfg.MigrateOnStart = cfg.Driver == "sqlite"
	if raw := strings.TrimSpace(os.Getenv("DB_MIGRATE_ON_START")); raw != "" {
		cfg.MigrateOnStart, err = strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("DB_MIGRATE_ON_START must be a boolean")
		}
	}
	return cfg, nil
}

func databaseInt(name string, fallback, minimum, maximum int) (int, error) {
	value, err := strconv.Atoi(envDefault(name, strconv.Itoa(fallback)))
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return value, nil
}

func databaseSecret() (string, error) {
	value := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	path := strings.TrimSpace(os.Getenv("DATABASE_URL_FILE"))
	if path == "" {
		return value, nil
	}
	if value != "" {
		return "", fmt.Errorf("set only one of DATABASE_URL and DATABASE_URL_FILE")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 {
		return "", fmt.Errorf("DATABASE_URL_FILE must name a readable regular secret file of at most 16 KiB")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read DATABASE_URL_FILE")
	}
	return strings.TrimSpace(string(raw)), nil
}
