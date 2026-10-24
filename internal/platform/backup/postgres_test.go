package backup_test

import (
	"context"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/datamigrate"
	"github.com/jackc/pgx/v5"
)

// Metadata is intentionally kept outside the frozen fixture so exact hashes
// describe the same instant as pg_dump's consistent snapshot.
type recoveryMetadata struct{}

func (recoveryMetadata) StartBackupRun(_ context.Context, path string, at time.Time) (model.BackupRun, error) {
	return model.BackupRun{ID: "recovery", Path: path, CreatedAt: at}, nil
}
func (recoveryMetadata) CompleteBackupRun(context.Context, string, int64, error, time.Time) error {
	return nil
}

func TestPostgresLogicalRecovery(t *testing.T) {
	url := os.Getenv("POSTGRES_RECOVERY_TEST_URL")
	if url == "" {
		t.Skip("hosted PostgreSQL recovery environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cfg := config.DatabaseConfig{Driver: "postgres", URL: url, MaxOpenConns: 4, MaxIdleConns: 2, ConnectTimeout: 10 * time.Second, StatementTimeout: time.Minute, LockTimeout: 5 * time.Second}
	db, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before, err := datamigrate.InspectPostgres(ctx, db, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	service := backup.NewService(db, recoveryMetadata{}, directory, 7*24*time.Hour)
	service.ConfigurePostgres(url)
	run, err := service.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(run.Path) != ".dump" || run.SizeBytes == 0 {
		t.Fatal("invalid logical archive metadata")
	}
	admin, err := pgx.Connect(ctx, os.Getenv("POSTGRES_RECOVERY_ADMIN_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	name := "vansail_logical_recovery"
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+name+` OWNER vansail_migration`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), `DROP DATABASE `+name+` WITH (FORCE)`)
	parsed, err := neturl.Parse(os.Getenv("POSTGRES_RECOVERY_MIGRATION_URL"))
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	restore := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname="+parsed.String(), run.Path)
	if output, e := restore.CombinedOutput(); e != nil {
		t.Fatalf("restore: %v %s", e, output)
	}
	// Recover owner/application permissions as a separate administrator step.
	// Database-local grants are not part of a --no-acl logical archive.
	adminConfig, err := pgx.ParseConfig(os.Getenv("POSTGRES_RECOVERY_ADMIN_URL"))
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.Database = name
	provision, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer provision.Close(ctx)
	if _, err = provision.Exec(ctx, `CREATE EXTENSION pg_stat_statements;
GRANT USAGE ON SCHEMA public TO vansail_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO vansail_app;
GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO vansail_app;
ALTER DEFAULT PRIVILEGES FOR ROLE vansail_migration IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO vansail_app;
ALTER DEFAULT PRIVILEGES FOR ROLE vansail_migration IN SCHEMA public GRANT USAGE,SELECT ON SEQUENCES TO vansail_app;`); err != nil {
		t.Fatal(err)
	}
	applicationURL, err := neturl.Parse(url)
	if err != nil {
		t.Fatal(err)
	}
	applicationURL.Path = "/" + name
	cfg.URL = applicationURL.String()
	restored, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	after, err := datamigrate.InspectPostgres(ctx, restored, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if before.Hash != after.Hash {
		t.Fatal("logical recovery changed authoritative records")
	}
	var role, owner string
	if err = restored.QueryRowContext(ctx, `SELECT current_user,pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.balances'::regclass`).Scan(&role, &owner); err != nil || role != "vansail_app" || owner != "vansail_migration" {
		t.Fatal("logical recovery role boundary changed", err)
	}
	t.Logf("restored %d exact tables, evidence %s", len(after.Tables), after.Hash)
}
