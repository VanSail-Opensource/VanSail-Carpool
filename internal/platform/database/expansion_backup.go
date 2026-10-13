package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	destructiveExpansionMigration = "009_expansion_and_cleanup.sql"
	expansionBackupRetention      = 7 * 24 * time.Hour
)

func backupBeforeExpansionMigration(ctx context.Context, db *sql.DB, databasePath string) error {
	var migrationTableCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&migrationTableCount); err != nil {
		return fmt.Errorf("inspect migration table before expansion: %w", err)
	}
	if migrationTableCount == 0 {
		return nil
	}
	var alreadyApplied, appliedCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, destructiveExpansionMigration).Scan(&alreadyApplied); err != nil {
		return fmt.Errorf("inspect expansion migration state: %w", err)
	}
	if alreadyApplied > 0 {
		return nil
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&appliedCount); err != nil {
		return fmt.Errorf("count applied migrations: %w", err)
	}
	if appliedCount == 0 {
		return nil
	}
	now := time.Now().UTC()
	if reusable, err := reuseExpansionBackup(ctx, databasePath, appliedCount, now); err != nil {
		return err
	} else if reusable {
		return nil
	}
	backupPath := databasePath + ".pre-009-" + now.Format("20060102T150405.000000000Z") + ".db"
	if _, err := db.ExecContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		return fmt.Errorf("checkpoint before expansion backup: %w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, backupPath); err != nil {
		return fmt.Errorf("create pre-expansion backup: %w", err)
	}
	if err := os.Chmod(backupPath, 0o600); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("protect pre-expansion backup: %w", err)
	}
	if err := verifyExpansionBackup(ctx, backupPath, appliedCount); err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	return nil
}

// PruneExpansionBackups removes adjacent pre-009 recovery snapshots after the
// seven-day rollback window and keeps retained snapshots owner-readable only.
func PruneExpansionBackups(databasePath string, now time.Time) error {
	paths, err := expansionBackupPaths(databasePath)
	if err != nil {
		return err
	}
	cutoff := now.UTC().Add(-expansionBackupRetention)
	for _, path := range paths {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("protect expansion backup: %w", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.ModTime().UTC().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("prune expansion backup: %w", err)
			}
		}
	}
	return nil
}

func reuseExpansionBackup(ctx context.Context, databasePath string, appliedCount int, now time.Time) (bool, error) {
	if err := PruneExpansionBackups(databasePath, now); err != nil {
		return false, err
	}
	paths, err := expansionBackupPaths(databasePath)
	if err != nil {
		return false, err
	}
	sort.Slice(paths, func(i, j int) bool {
		left, _ := os.Stat(paths[i])
		right, _ := os.Stat(paths[j])
		return left.ModTime().After(right.ModTime())
	})
	retained := ""
	for _, path := range paths {
		if retained == "" && verifyExpansionBackup(ctx, path, appliedCount) == nil {
			retained = path
			continue
		}
		if err := os.Remove(path); err != nil {
			return false, fmt.Errorf("remove duplicate expansion backup: %w", err)
		}
	}
	return retained != "", nil
}

func expansionBackupPaths(databasePath string) ([]string, error) {
	directory := filepath.Dir(databasePath)
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	prefix := filepath.Base(databasePath) + ".pre-009-"
	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	return paths, nil
}

func verifyExpansionBackup(ctx context.Context, path string, appliedCount int) error {
	return verifyMigrationBoundaryBackup(ctx, path, appliedCount, destructiveExpansionMigration)
}

func verifyMigrationBoundaryBackup(ctx context.Context, path string, appliedCount int, pendingMigration string) error {
	backupDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return fmt.Errorf("open pre-expansion backup: %w", err)
	}
	defer func() { _ = backupDB.Close() }()
	var integrity string
	if err := backupDB.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("verify pre-expansion backup: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("pre-expansion backup integrity result: %s", integrity)
	}
	var count, applied int
	if err := backupDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		return fmt.Errorf("inspect pre-expansion migrations: %w", err)
	}
	if err := backupDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, pendingMigration).Scan(&applied); err != nil {
		return fmt.Errorf("inspect pre-expansion marker: %w", err)
	}
	if count != appliedCount || applied != 0 {
		return errors.New("pre-expansion backup does not match the pending migration boundary")
	}
	return nil
}
