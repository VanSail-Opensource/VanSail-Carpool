// Package datamigrate moves exact authoritative records from a verified SQLite
// snapshot into a fresh PostgreSQL database. It never mutates the source.
package datamigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// Source holds one immutable read snapshot plus its read-only connection pool.
type Source struct {
	DB     *sql.DB
	Tx     *sql.Tx
	Tables []postgres.TableContract
}

// OpenSource validates schema version, integrity, foreign keys and identity rekey.
func OpenSource(ctx context.Context, path string) (*Source, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	filename := (&url.URL{Path: filepath.ToSlash(abs)}).EscapedPath()
	db, err := sql.Open("sqlite", "file:"+filename+"?mode=ro&_pragma=query_only(1)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &Source{DB: db, Tx: tx}
	if err = s.validate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the read snapshot without committing any mutation.
func (s *Source) Close() error { s.Tx.Rollback(); return s.DB.Close() }

func (s *Source) validate(ctx context.Context) error {
	var check string
	if err := s.Tx.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("source integrity check failed")
	}
	rows, err := s.Tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	invalid := rows.Next()
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if invalid {
		return fmt.Errorf("source foreign key violations")
	}
	expected, err := database.MigrationVersions()
	if err != nil {
		return err
	}
	rows, err = s.Tx.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return err
	}
	index := 0
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			rows.Close()
			return err
		}
		if index >= len(expected) || expected[index] != version {
			rows.Close()
			return fmt.Errorf("unsupported source migration history")
		}
		index++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if index != len(expected) {
		return fmt.Errorf("source migrations are incomplete")
	}
	rows, err = s.Tx.QueryContext(ctx, `SELECT type,name,sql FROM sqlite_schema WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'index' THEN 1 ELSE 2 END,name`)
	if err != nil {
		return err
	}
	hash := sha256.New()
	for rows.Next() {
		var kind, name, definition string
		if err = rows.Scan(&kind, &name, &definition); err != nil {
			rows.Close()
			return err
		}
		fmt.Fprintf(hash, "%s\x00%s\x00%s\x00", kind, name, strings.ReplaceAll(definition, "\r", ""))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != postgres.SourceSchemaHash() {
		return fmt.Errorf("source schema differs from the final post-startup baseline; complete identity migration with the compatible source release")
	}
	contract, err := postgres.SchemaContract()
	if err != nil {
		return err
	}
	byName := map[string]postgres.TableContract{}
	for _, table := range contract.Tables {
		byName[table.Name] = table
	}
	for _, name := range contract.Order {
		s.Tables = append(s.Tables, byName[name])
	}
	var bad bool
	if err = s.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id<>CAST(telegram_id AS TEXT) OR telegram_id<=0 OR telegram_id>=4503599627370496) OR EXISTS(SELECT 1 FROM xnode_identities WHERE proxy_user_id<=0 OR proxy_user_id>=9007199254740992 OR (user_id IS NOT NULL AND proxy_user_id<>CAST(user_id AS INTEGER)) OR (user_id IS NULL AND proxy_user_id<4503599627370496))`).Scan(&bad); err != nil {
		return err
	}
	if bad {
		return fmt.Errorf("source identity rekey is incomplete")
	}
	return nil
}

func quote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
