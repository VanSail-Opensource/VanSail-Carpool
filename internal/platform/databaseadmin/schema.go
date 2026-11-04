package databaseadmin

import (
	"context"
	"database/sql"

	"regexp"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type storageFamily uint8

const (
	storageFamilyBlob storageFamily = iota
	storageFamilyText
	storageFamilyInteger
	storageFamilyReal
	storageFamilyNumeric
)

type schemaColumn struct {
	Column
	Family  storageFamily
	Boolean bool
	Default sql.NullString
	Hidden  int
}

type tableSchema struct {
	Table
	SQL        string
	ColumnsRaw []schemaColumn
	KeyColumns []schemaColumn
}

// Tables returns the allowlisted application tables from PostgreSQL catalogs.

func (s *Service) Tables(ctx context.Context) ([]Table, error) {

	return s.postgresTables(ctx)

}

func (s *Service) schema(ctx context.Context, table string) (tableSchema, error) {

	return s.postgresSchema(ctx, table)

}

func quoteIdentifier(value string) string {
	// Callers must first resolve value from PostgreSQL catalogs and validate it with
	// identifierPattern. Double quoting keeps reserved words addressable.
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func columnFamily(declared string) storageFamily {
	if strings.HasPrefix(strings.ToLower(declared), "numeric(") || strings.HasPrefix(strings.ToLower(declared), "decimal(") {
		return storageFamilyNumeric
	}
	switch strings.ToLower(declared) {
	case "bigint", "integer", "smallint":
		return storageFamilyInteger
	case "real", "double precision":
		return storageFamilyReal
	case "bytea":
		return storageFamilyBlob
	case "numeric", "decimal":
		return storageFamilyNumeric
	default:
		return storageFamilyText
	}
}
