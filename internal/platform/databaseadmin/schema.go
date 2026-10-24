package databaseadmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"regexp"
	"sort"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type affinity uint8

const (
	affinityBlob affinity = iota
	affinityText
	affinityInteger
	affinityReal
	affinityNumeric
)

type schemaColumn struct {
	Column
	Affinity affinity
	Boolean  bool
	Default  sql.NullString
	Hidden   int
}

type tableSchema struct {
	Table
	SQL        string
	ColumnsRaw []schemaColumn
	KeyColumns []schemaColumn
	UsesRowID  bool
	RowIDName  string
}

// Tables returns every application table except schema_migrations and SQLite
// internals. Identifiers are sourced exclusively from sqlite_schema.

func (s *Service) Tables(ctx context.Context) ([]Table, error) {
	if postgres.Is(s.db.Driver()) {
		return s.postgresTables(ctx)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT name,sql FROM sqlite_schema
		WHERE type='table' AND name<>'schema_migrations' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list application tables: %w", err)
	}
	defer rows.Close()
	items := make([]Table, 0)
	for rows.Next() {
		var name, createSQL string
		if err := rows.Scan(&name, &createSQL); err != nil {
			return nil, fmt.Errorf("scan application table: %w", err)
		}
		schema, err := s.introspect(ctx, name, createSQL)
		if err != nil {
			return nil, err
		}
		items = append(items, schema.Table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application tables: %w", err)
	}
	return items, nil
}

func (s *Service) schema(ctx context.Context, table string) (tableSchema, error) {
	if postgres.Is(s.db.Driver()) {
		return s.postgresSchema(ctx, table)
	}
	if !identifierPattern.MatchString(table) || table == "schema_migrations" || strings.HasPrefix(strings.ToLower(table), "sqlite_") {
		return tableSchema{}, ErrTableNotFound
	}
	var name, createSQL string
	err := s.db.QueryRowContext(ctx, `SELECT name,sql FROM sqlite_schema WHERE type='table' AND name=? AND name<>'schema_migrations' AND name NOT LIKE 'sqlite_%'`, table).Scan(&name, &createSQL)
	if errors.Is(err, sql.ErrNoRows) {
		return tableSchema{}, ErrTableNotFound
	}
	if err != nil {
		return tableSchema{}, fmt.Errorf("lookup application table: %w", err)
	}
	if name != table || !identifierPattern.MatchString(name) {
		return tableSchema{}, ErrTableNotFound
	}
	return s.introspect(ctx, name, createSQL)
}

func (s *Service) introspect(ctx context.Context, name, createSQL string) (tableSchema, error) {
	if !identifierPattern.MatchString(name) {
		return tableSchema{}, fmt.Errorf("%w: schema returned unsafe identifier", ErrTableNotFound)
	}
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_xinfo(`+quoteIdentifier(name)+`)`)
	if err != nil {
		return tableSchema{}, fmt.Errorf("inspect table %s: %w", name, err)
	}
	defer rows.Close()
	columns := make([]schemaColumn, 0)
	for rows.Next() {
		var cid, notNull, primaryKey, hidden int
		var columnName, declaredType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &declaredType, &notNull, &defaultValue, &primaryKey, &hidden); err != nil {
			return tableSchema{}, fmt.Errorf("scan table %s column: %w", name, err)
		}
		if cid < 0 || !identifierPattern.MatchString(columnName) {
			return tableSchema{}, fmt.Errorf("%w: table %s contains unsafe column", ErrTableNotFound, name)
		}
		boolean := isBooleanColumn(columnName, declaredType)
		sensitive := isSensitiveColumn(name, columnName)
		writeOnlySetting := name == "settings" && columnName == "value"
		column := schemaColumn{
			Column: Column{Name: columnName, DeclaredType: declaredType, Nullable: notNull == 0 && primaryKey == 0,
				PrimaryKeyPosition: primaryKey, Editable: hidden == 0 && (!sensitive || writeOnlySetting), Sensitive: sensitive},
			Affinity: columnAffinity(declaredType), Boolean: boolean, Default: defaultValue, Hidden: hidden,
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return tableSchema{}, fmt.Errorf("iterate table %s columns: %w", name, err)
	}
	if len(columns) == 0 {
		return tableSchema{}, fmt.Errorf("%w: table %s has no visible schema", ErrTableNotFound, name)
	}
	keys := make([]schemaColumn, 0)
	for _, column := range columns {
		if column.PrimaryKeyPosition > 0 {
			keys = append(keys, column)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].PrimaryKeyPosition < keys[j].PrimaryKeyPosition })
	withoutRowID := strings.Contains(strings.ToUpper(createSQL), "WITHOUT ROWID")
	rowIDShadowed := make(map[string]bool, 3)
	for _, column := range columns {
		switch strings.ToLower(column.Name) {
		case "rowid", "_rowid_", "oid":
			rowIDShadowed[strings.ToLower(column.Name)] = true
		}
	}
	rowIDName := ""
	if len(keys) == 0 && !withoutRowID {
		// SQLite exposes the hidden row identifier through any unshadowed one of
		// these three aliases. A user column shadows only its matching alias, not
		// all of them, so keep otherwise-addressable tables editable.
		for _, candidate := range []string{"_rowid_", "rowid", "oid"} {
			if !rowIDShadowed[candidate] {
				rowIDName = candidate
				break
			}
		}
	}
	usesRowID := rowIDName != ""
	if len(keys) == 0 && !usesRowID {
		return tableSchema{}, fmt.Errorf("%w: table %s has neither a declared primary key nor an addressable rowid", ErrTableNotFound, name)
	}
	publicColumns := make([]Column, 0, len(columns))
	for _, column := range columns {
		publicColumns = append(publicColumns, column.Column)
	}
	return tableSchema{
		Table: Table{Name: name, Columns: publicColumns, HighRisk: true, SupportsRowID: usesRowID, Warning: bypassWarning},
		SQL:   createSQL, ColumnsRaw: columns, KeyColumns: keys, UsesRowID: usesRowID, RowIDName: rowIDName,
	}, nil
}

func quoteIdentifier(value string) string {
	// Callers must first resolve value from sqlite_schema and validate it with
	// identifierPattern. Double quoting keeps reserved words addressable.
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func columnAffinity(declared string) affinity {
	declared = strings.ToUpper(declared)
	switch {
	case strings.Contains(declared, "INT"):
		return affinityInteger
	case strings.Contains(declared, "CHAR"), strings.Contains(declared, "CLOB"), strings.Contains(declared, "TEXT"):
		return affinityText
	case strings.Contains(declared, "BLOB") || declared == "BYTEA" || strings.TrimSpace(declared) == "":
		return affinityBlob
	case strings.Contains(declared, "REAL"), strings.Contains(declared, "FLOA"), strings.Contains(declared, "DOUB"):
		return affinityReal
	default:
		return affinityNumeric
	}
}
