package datamigrate

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// SQLite permits a null TEXT primary key; PostgreSQL correctly refuses it.
// Force a mid-copy constraint failure and prove the original target survives.
func assertImportRollback(t *testing.T, ctx context.Context, snapshot, url string, target *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "invalid-source.db")
	raw, err := os.ReadFile(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at) VALUES(NULL,'invalid',0,'2000-01-01T00:00:00Z')`)
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	source, err := OpenSource(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err = Import(ctx, source, url); err == nil {
		t.Fatal("invalid primary key was accepted")
	}
	var phase string
	if err = target.QueryRowContext(ctx, `SELECT phase FROM vansail_migration_state WHERE singleton=1`).Scan(&phase); err != nil || phase != "fresh" {
		t.Fatal("failed import changed activation state", err)
	}
	var users int64
	if err = target.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil || users != 0 {
		t.Fatal("failed import retained partial data", err)
	}
	if err = postgres.VerifySchema(ctx, target); err != nil {
		t.Fatal("failed import did not restore triggers", err)
	}
}
