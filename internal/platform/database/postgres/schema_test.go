package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
)

func TestPostgresBaseline(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("hosted PostgreSQL integration environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cfg := config.DatabaseConfig{Driver: "postgres", URL: url, MaxOpenConns: 4, MaxIdleConns: 2, ConnectTimeout: 10 * time.Second, StatementTimeout: time.Minute, LockTimeout: 5 * time.Second, MigrateOnStart: true}
	db, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = VerifySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("idempotent migration:", err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE affiliate_tiers SET name='tampered'`); err == nil {
		t.Fatal("immutable tier accepted update")
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM affiliate_config_versions`); err == nil {
		t.Fatal("immutable config accepted deletion")
	}
	var timestamp string
	if err = db.QueryRowContext(ctx, `SELECT $1::text`, "2026-10-24T01:02:03.123456789Z").Scan(&timestamp); err != nil || timestamp != "2026-10-24T01:02:03.123456789Z" {
		t.Fatal("timestamp evidence changed", err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO goose_db_version(version_id,is_applied) VALUES(999,true)`); err != nil {
		t.Fatal(err)
	}
	if err = VerifyVersion(ctx, db); err == nil {
		t.Fatal("future schema accepted")
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM goose_db_version WHERE version_id=999`); err != nil {
		t.Fatal(err)
	}
}
