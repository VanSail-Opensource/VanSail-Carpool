// Package dbinventory records a consistent, read-only SQLite schema inventory.
package dbinventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Object preserves the actual deployed SQL, including partial indexes and triggers.
type Object struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Table string `json:"table"`
	SQL   string `json:"sql"`
}

// Column records the complete SQLite column contract for baseline generation.
type Column struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	NotNull    bool    `json:"notNull"`
	Default    *string `json:"default"`
	PrimaryKey int     `json:"primaryKey"`
}

// Table records authoritative columns and the row count of one consistent snapshot.
type Table struct {
	Name    string   `json:"name"`
	Rows    int64    `json:"rows"`
	Columns []Column `json:"columns"`
}

// Inventory is deterministic for a given schema and dataset. It contains no row values.
type Inventory struct {
	Migrations []string `json:"migrations"`
	Objects    []Object `json:"objects"`
	Tables     []Table  `json:"tables"`
}

// Capture never migrates or writes the source. All reads share a read transaction.
func Capture(ctx context.Context, source, output string) error {
	abs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	filename := (&url.URL{Path: filepath.ToSlash(abs)}).EscapedPath()
	db, err := sql.Open("sqlite", "file:"+filename+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var check string
	if err = tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("source integrity: %s", check)
	}
	inv, err := read(ctx, tx)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err = os.MkdirAll(output, 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "sqlite-inventory.json"), append(raw, '\n'), 0o600); err != nil {
		return err
	}
	var schema strings.Builder
	schema.WriteString("-- Generated from database.Open plus completed Telegram identity migration.\n-- Historical SQLite migration files remain immutable.\n")
	for _, object := range inv.Objects {
		schema.WriteString(object.SQL + ";\n\n")
	}
	return os.WriteFile(filepath.Join(output, "sqlite-schema.sql"), []byte(schema.String()), 0o600)
}

func read(ctx context.Context, tx *sql.Tx) (Inventory, error) {
	var inv Inventory
	rows, err := tx.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return inv, err
	}
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			rows.Close()
			return inv, err
		}
		inv.Migrations = append(inv.Migrations, version)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return inv, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT type,name,tbl_name,sql FROM sqlite_schema WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'index' THEN 1 ELSE 2 END,name`)
	if err != nil {
		return inv, err
	}
	for rows.Next() {
		var object Object
		if err = rows.Scan(&object.Type, &object.Name, &object.Table, &object.SQL); err != nil {
			rows.Close()
			return inv, err
		}
		object.SQL = strings.ReplaceAll(object.SQL, "\r", "")
		inv.Objects = append(inv.Objects, object)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return inv, err
	}
	for _, object := range inv.Objects {
		if object.Type != "table" {
			continue
		}
		table, e := readTable(ctx, tx, object.Name)
		if e != nil {
			return inv, e
		}
		inv.Tables = append(inv.Tables, table)
	}
	return inv, nil
}

func readTable(ctx context.Context, tx *sql.Tx, name string) (Table, error) {
	table := Table{Name: name}
	quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoted).Scan(&table.Rows); err != nil {
		return table, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT name,type,\"notnull\",dflt_value,pk FROM pragma_table_xinfo(?) ORDER BY cid", name)
	if err != nil {
		return table, err
	}
	defer rows.Close()
	for rows.Next() {
		var column Column
		if err := rows.Scan(&column.Name, &column.Type, &column.NotNull, &column.Default, &column.PrimaryKey); err != nil {
			return table, err
		}
		table.Columns = append(table.Columns, column)
	}
	return table, rows.Err()
}
