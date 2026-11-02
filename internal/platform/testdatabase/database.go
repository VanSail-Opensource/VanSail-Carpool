// Package testdatabase creates isolated PostgreSQL fixtures for hosted checks.
package testdatabase

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/google/uuid"
)

var template struct {
	sync.Once
	url string
	err error
}

// Open clones a migrated template and removes its private database at cleanup.
// It requires hosted PostgreSQL credentials and never selects another backend.
func Open(ctx context.Context, t testing.TB) (*sql.DB, error) {
	t.Helper()
	template.Do(func() { template.url, template.err = prepareTemplate(ctx) })
	if template.err != nil {
		return nil, template.err
	}
	parsed, err := url.Parse(template.url)
	if err != nil {
		return nil, err
	}
	source := strings.TrimPrefix(parsed.Path, "/")
	adminURL := *parsed
	adminURL.Path = "/postgres"
	admin, err := sql.Open("pgx", adminURL.String())
	if err != nil {
		return nil, err
	}
	name := databaseName()
	if _, err = admin.ExecContext(ctx, `CREATE DATABASE `+quote(name)+` TEMPLATE `+quote(source)); err != nil {
		admin.Close()
		return nil, fmt.Errorf("clone hosted PostgreSQL fixture: %w", err)
	}
	parsed.Path = "/" + name
	db, err := postgres.Open(ctx, configuration(parsed.String(), false))
	t.Cleanup(func() {
		if db != nil {
			db.Close()
		}
		cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, dropErr := admin.ExecContext(cleanup, `DROP DATABASE `+quote(name)+` WITH (FORCE)`); dropErr != nil {
			t.Errorf("remove hosted PostgreSQL fixture: %v", dropErr)
		}
		admin.Close()
	})
	return db, err
}

func configuration(connectionURL string, migrate bool) config.DatabaseConfig {
	return config.DatabaseConfig{Driver: "postgres", URL: connectionURL,
		MaxOpenConns: 8, MaxIdleConns: 2, ConnectTimeout: 15 * time.Second,
		StatementTimeout: time.Minute, LockTimeout: 10 * time.Second, MigrateOnStart: migrate}
}

func databaseName() string      { return "vansail_fixture_" + strings.ReplaceAll(uuid.NewString(), "-", "") }
func quote(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }
