package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const usdtMigration = "072_builtin_usdt.sql"

// migrateUSDT rebuilds the referenced payment table with FK enforcement restored
// before the dedicated connection is returned to the pool. No writable_schema edits.
func migrateUSDT(ctx context.Context, db *sql.DB, script []byte) (result error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil && result == nil {
			result = err
		}
	}()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(restoreCtx, "PRAGMA foreign_keys=ON"); err != nil {
			result = fmt.Errorf("restore foreign keys: %w", err)
		}
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	invalid := rows.Next()
	rowErr := rows.Err()
	closeErr := rows.Close()
	if invalid || rowErr != nil || closeErr != nil {
		return fmt.Errorf("USDT migration foreign key validation failed")
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", usdtMigration, stamp(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}

func backupBeforeUSDTMigration(ctx context.Context, db *sql.DB, path string) error {
	var migrationsPresent int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name='schema_migrations'").Scan(&migrationsPresent); err != nil {
		return err
	}
	if migrationsPresent == 0 {
		return nil
	}
	var applied, count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version=?", usdtMigration).Scan(&applied); err != nil {
		return err
	}
	if applied > 0 {
		return nil
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	backupPath := path + ".pre-usdt-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".db"
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", backupPath); err != nil {
		return fmt.Errorf("backup before USDT migration: %w", err)
	}
	if err := os.Chmod(backupPath, 0o600); err != nil {
		return err
	}
	return verifyMigrationBoundaryBackup(ctx, backupPath, count, usdtMigration)
}

func pruneUSDTMigrationBackups(databasePath string, now time.Time) error {
	entries, err := os.ReadDir(filepath.Dir(databasePath))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	prefix := filepath.Base(databasePath) + ".pre-usdt-"
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".db") || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && now.Sub(info.ModTime()) > expansionBackupRetention {
			if err := os.Remove(filepath.Join(filepath.Dir(databasePath), entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
