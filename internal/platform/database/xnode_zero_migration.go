package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
)

const zeroMultiplierMigration = "083_xnode_zero_multiplier.sql"

func backupBeforeZeroMultiplierMigration(ctx context.Context, db *sql.DB, path string) error {
	var tables int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='xnode_nodes'").Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil
	}
	var applied, count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=?", zeroMultiplierMigration).Scan(&applied); err != nil {
		return err
	}
	if applied > 0 {
		return nil
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		return err
	}
	backup := path + ".pre-xnode-zero-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".db"
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", backup); err != nil {
		return fmt.Errorf("backup before zero multiplier migration: %w", err)
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		return err
	}
	return verifyMigrationBoundaryBackup(ctx, backup, count, zeroMultiplierMigration)
}
