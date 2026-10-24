package database

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/google/uuid"
)

// Hosted integration tests clone a fully migrated, inactive template. Each test
// owns a separate database. Production never reads this test-only environment.
func openPostgresContractDatabase(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	parsed, err := url.Parse(os.Getenv("POSTGRES_CONTRACT_URL"))
	if err != nil {
		t.Fatal(err)
	}
	adminURL := *parsed
	adminURL.Path = "/postgres"
	admin, err := sql.Open("pgx", adminURL.String())
	if err != nil {
		t.Fatal(err)
	}
	admin.SetMaxOpenConns(1)
	name := "vansail_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.ExecContext(ctx, `CREATE DATABASE `+quoteSQLName(name)+` TEMPLATE `+quoteSQLName(strings.TrimPrefix(parsed.Path, "/"))); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	cfg := config.DatabaseConfig{Driver: "postgres", URL: parsed.String(), MaxOpenConns: 8, MaxIdleConns: 2, ConnectTimeout: 10 * time.Second, StatementTimeout: 30 * time.Second, LockTimeout: 5 * time.Second}
	db, err := postgres.Open(ctx, cfg)
	t.Cleanup(func() {
		if db != nil {
			db.Close()
		}
		cleanup, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		if _, err := admin.ExecContext(cleanup, `DROP DATABASE `+quoteSQLName(name)+` WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}
