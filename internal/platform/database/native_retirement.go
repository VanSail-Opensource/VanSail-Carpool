package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
)

const nativeRetirementMigration = "093_native_only.sql"

// A verified boundary backup precedes deletion of completed bridge artifacts.
func backupBeforeNativeRetirement(ctx context.Context, db *sql.DB, path string) error {
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&tables); err != nil || tables == 0 {
		return err
	}
	var applied, count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, nativeRetirementMigration).Scan(&applied); err != nil || applied > 0 {
		return err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count == 0 {
		return err
	}
	backup := path + ".pre-native-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".db"
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, backup); err != nil {
		return fmt.Errorf("backup before native retirement: %w", err)
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		return err
	}
	return verifyMigrationBoundaryBackup(ctx, backup, count, nativeRetirementMigration)
}
