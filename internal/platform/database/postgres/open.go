// Package postgres owns PostgreSQL connections and the independent Goose schema.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// SchemaVersion is the newest PostgreSQL schema understood by this binary.
const SchemaVersion int64 = 10

// Open opens exactly the configured PostgreSQL backend and enforces schema policy.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*sql.DB, error) {
	parsed, err := pgx.ParseConfig(cfg.URL)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL connection configuration")
	}
	parsed.ConnectTimeout = cfg.ConnectTimeout
	parsed.RuntimeParams["timezone"] = "UTC"
	parsed.RuntimeParams["application_name"] = "vansail-carpool"
	parsed.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	parsed.RuntimeParams["lock_timeout"] = strconv.FormatInt(cfg.LockTimeout.Milliseconds(), 10)
	parsed.RuntimeParams["idle_in_transaction_session_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	db := sql.OpenDB(connector{Connector: stdlib.GetConnector(*parsed), metrics: &transactionMetrics{}})
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	connect, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err = db.PingContext(connect); err != nil {
		db.Close()
		return nil, errors.New("PostgreSQL connection failed; inspect server connectivity and authentication")
	}
	if cfg.MigrateOnStart {
		err = Migrate(ctx, db)
	} else {
		err = VerifyVersion(ctx, db)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate applies forward-only migrations under a session-scoped advisory lock.
func Migrate(ctx context.Context, db *sql.DB) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.goose_db_version') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		var newest int64
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied`).Scan(&newest); err != nil {
			return err
		}
		if newest > SchemaVersion {
			return fmt.Errorf("PostgreSQL schema %d is newer than supported %d", newest, SchemaVersion)
		}
	}
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithDisableGlobalRegistry(true), goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	if _, err = provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate PostgreSQL schema: %w", err)
	}
	return VerifyVersion(ctx, db)
}

// VerifyVersion refuses uninitialized, incomplete and future schema histories.
func VerifyVersion(ctx context.Context, db *sql.DB) error {
	var count, newest int64
	err := db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT version_id),COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied AND version_id>0`).Scan(&count, &newest)
	if err != nil {
		return errors.New("PostgreSQL schema is not initialized; run the migration command with the migration role")
	}
	if count != SchemaVersion || newest != SchemaVersion {
		return fmt.Errorf("unsupported PostgreSQL schema history: count=%d latest=%d expected=%d", count, newest, SchemaVersion)
	}
	return nil
}
