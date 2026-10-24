package databaseadmin

import (
	"context"
	"fmt"
	"sort"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func (s *Service) postgresReadOnly() bool { return postgres.Is(s.db.Driver()) }

func (s *Service) postgresTables(ctx context.Context) ([]Table, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r' AND c.relname NOT IN ('schema_migrations','goose_db_version') ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	items := make([]Table, 0, len(names))
	for _, name := range names {
		schema, e := s.postgresSchema(ctx, name)
		if e != nil {
			return nil, e
		}
		items = append(items, schema.Table)
	}
	return items, nil
}

func (s *Service) postgresSchema(ctx context.Context, name string) (tableSchema, error) {
	if !identifierPattern.MatchString(name) || name == "schema_migrations" || name == "goose_db_version" {
		return tableSchema{}, ErrTableNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,pg_get_expr(d.adbin,d.adrelid),COALESCE((SELECT array_position(k.conkey,a.attnum) FROM pg_constraint k WHERE k.conrelid=c.oid AND k.contype='p'),0) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum WHERE n.nspname='public' AND c.relname=? AND c.relkind='r' AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, name)
	if err != nil {
		return tableSchema{}, err
	}
	defer rows.Close()
	schema := tableSchema{Table: Table{Name: name, HighRisk: true, Warning: bypassWarning}}
	for rows.Next() {
		var c schemaColumn
		var notNull bool
		if err = rows.Scan(&c.Name, &c.DeclaredType, &notNull, &c.Default, &c.PrimaryKeyPosition); err != nil {
			return tableSchema{}, err
		}
		if !identifierPattern.MatchString(c.Name) {
			return tableSchema{}, fmt.Errorf("unsafe catalog column")
		}
		c.Nullable = !notNull
		c.Editable = false
		c.Sensitive = isSensitiveColumn(name, c.Name)
		c.Affinity = columnAffinity(c.DeclaredType)
		c.Boolean = isBooleanColumn(c.Name, c.DeclaredType)
		schema.ColumnsRaw = append(schema.ColumnsRaw, c)
		schema.Columns = append(schema.Columns, c.Column)
		if c.PrimaryKeyPosition > 0 {
			schema.KeyColumns = append(schema.KeyColumns, c)
		}
	}
	if err = rows.Err(); err != nil {
		return tableSchema{}, err
	}
	if len(schema.ColumnsRaw) == 0 {
		return tableSchema{}, ErrTableNotFound
	}
	sort.Slice(schema.KeyColumns, func(i, j int) bool {
		return schema.KeyColumns[i].PrimaryKeyPosition < schema.KeyColumns[j].PrimaryKeyPosition
	})
	if len(schema.KeyColumns) == 0 {
		return tableSchema{}, fmt.Errorf("%w: PostgreSQL requires a declared primary key", ErrTableNotFound)
	}
	return schema, nil
}
