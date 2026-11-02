package postgres

import (
	"context"
	"database/sql"
	"io/fs"
	neturl "net/url"
	"os"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestPostgresForwardUpgrade(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_ADMIN_URL")
	if url == "" {
		t.Skip("hosted isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, `CREATE DATABASE vansail_upgrade_rehearsal OWNER vansail_migration`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), `DROP DATABASE vansail_upgrade_rehearsal WITH (FORCE)`)
	targetURL, err := neturl.Parse(os.Getenv("POSTGRES_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	targetURL.Path = "/vansail_upgrade_rehearsal"
	parsed, err := pgx.ParseConfig(targetURL.String())
	if err != nil {
		t.Fatal(err)
	}
	raw := sql.OpenDB(stdlib.GetConnector(*parsed))
	raw.SetMaxOpenConns(4)
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		raw.Close()
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, raw, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 3); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if err = VerifyVersion(ctx, raw); err == nil {
		raw.Close()
		t.Fatal("old schema was accepted without upgrade")
	}
	if _, err = provider.UpTo(ctx, 17); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	seedRetirementUpgrade(t, ctx, raw)
	seedCommunityRetirement(t, ctx, raw)
	if _, err = provider.UpTo(ctx, 31); err != nil {
		t.Fatal(err)
	}
	contract, err := SchemaContract()
	if err != nil {
		t.Fatal(err)
	}
	beforeNative, err := inspect(ctx, raw, contract.Tables)
	if err != nil {
		t.Fatal(err)
	}
	if err = raw.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := config.DatabaseConfig{Driver: "postgres", URL: targetURL.String(), MaxOpenConns: 4, MaxIdleConns: 2, ConnectTimeout: 10 * time.Second, StatementTimeout: time.Minute, LockTimeout: 5 * time.Second, MigrateOnStart: true}
	db, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	afterNative, err := inspect(ctx, db, contract.Tables)
	if err != nil {
		t.Fatal(err)
	}
	if beforeNative.Hash != afterNative.Hash {
		t.Fatal("native upgrade changed authoritative records or identity sequences")
	}
	var obsolete int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname IN ('json_valid','json_type','json_extract','json_each','json_set','json_patch','vansail_json','julianday','instr')`).Scan(&obsolete); err != nil || obsolete != 0 {
		t.Fatal("live compatibility function retained", obsolete, err)
	}

	verifyRetirementUpgrade(t, ctx, db)
	verifyCommunityRetirement(t, ctx, db)
	if err = VerifySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("upgraded schema is not idempotent", err)
	}
}
