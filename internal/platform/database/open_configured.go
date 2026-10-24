package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// OpenConfigured opens only the explicitly selected authoritative backend.
func OpenConfigured(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	switch cfg.Database.Driver {
	case "postgres":
		return postgres.Open(ctx, cfg.Database)
	case "sqlite", "":
		db, err := openSQLite(ctx, cfg.DatabasePath, cfg.Database.Driver == "" || cfg.Database.MigrateOnStart)
		if err != nil {
			return nil, err
		}
		if cfg.Database.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
			db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
			db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported database driver")
	}
}

// IsPostgres identifies the backend without running a query or caching DSNs.
func IsPostgres(db *sql.DB) bool { return postgres.Is(db.Driver()) }

func verifySQLiteVersion(ctx context.Context, db *sql.DB) error {
	expected, err := MigrationVersions()
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("SQLite schema requires initialization: %w", err)
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			return err
		}
		if index >= len(expected) || expected[index] != version {
			return fmt.Errorf("unsupported SQLite schema history")
		}
		index++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if index != len(expected) {
		return fmt.Errorf("SQLite schema migrations are pending")
	}
	return nil
}
