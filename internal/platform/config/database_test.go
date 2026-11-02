package config

import (
	"strings"
	"testing"
)

// These cases run only in hosted CI under the repository test policy.
func TestDatabaseConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name      string
		env       map[string]string
		wantError string
	}{
		{"missing-default", nil, "DATABASE_URL"},
		{"postgres", map[string]string{"DB_DRIVER": "postgres", "DATABASE_URL": "postgres://app:secret@db/vansail"}, ""},
		{"no-fallback", map[string]string{"DB_DRIVER": "mysql"}, "DB_DRIVER"},
		{"default-postgres", map[string]string{"DATABASE_URL": "postgres://app:secret@db/vansail"}, ""},
		{"missing-url", map[string]string{"DB_DRIVER": "postgres"}, "DATABASE_URL"},
		{"redacted-url", map[string]string{"DB_DRIVER": "postgres", "DATABASE_URL": "postgres://app:TOPSECRET%zz@db/vansail"}, "DATABASE_URL"},
		{"pool-bounds", map[string]string{"DB_MAX_OPEN_CONNS": "2", "DB_MAX_IDLE_CONNS": "3"}, "DB_MAX_IDLE_CONNS"},
		{"lock-deadline", map[string]string{"DB_LOCK_TIMEOUT": "1m", "DB_STATEMENT_TIMEOUT": "1s"}, "DB_LOCK_TIMEOUT"},
		{"migration-policy", map[string]string{"DB_MIGRATE_ON_START": "maybe"}, "DB_MIGRATE_ON_START"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"DB_DRIVER", "DATABASE_URL", "DATABASE_URL_FILE", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONNECT_TIMEOUT", "DB_STATEMENT_TIMEOUT", "DB_LOCK_TIMEOUT", "DB_MIGRATE_ON_START"} {
				t.Setenv(name, "")
			}
			if tc.name != "missing-default" && tc.name != "missing-url" {
				t.Setenv("DATABASE_URL", "postgres://app:secret@db/vansail")
			}
			for name, value := range tc.env {
				t.Setenv(name, value)
			}
			cfg, err := loadDatabase()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected %s validation error", tc.wantError)
				}
				if strings.Contains(err.Error(), "TOPSECRET") {
					t.Fatal("configuration error disclosed credentials")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.MigrateOnStart || cfg.Driver != "postgres" {
				t.Fatal("unsafe default migration policy")
			}
		})
	}
}
