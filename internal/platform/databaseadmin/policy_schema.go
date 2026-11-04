package databaseadmin

import "context"

func (s *Service) applySchemaPolicy(ctx context.Context, schema *tableSchema) error {
	reason := tableProtection(schema.Name)
	schema.ProtectionReason = reason
	schema.AllowInsert, schema.AllowUpdate, schema.AllowDelete = reason == "", reason == "", reason == ""
	for index := range schema.ColumnsRaw {
		column := &schema.ColumnsRaw[index]
		column.ProtectionReason = columnProtection(schema.Name, *column, false)
		column.Editable = column.ProtectionReason == ""
		column.Insertable = columnProtection(schema.Name, *column, true) == ""
		column.HasDefault = column.Default.Valid
		if column.Sensitive {
			schema.AllowDelete = false
		}
		if !column.Insertable && !column.Nullable && !column.Default.Valid && column.Name != "created_at" && column.Name != "updated_at" {
			schema.AllowInsert = false
		}
		schema.Columns[index] = column.Column
	}
	if reason != "" || !schema.AllowDelete {
		return nil
	}
	// A generic delete must never cascade into protected evidence or credentials.
	// Native constraints remain authoritative for all other dependency checks.
	rows, err := s.db.QueryContext(ctx, `WITH RECURSIVE affected(oid) AS (
 SELECT to_regclass($1)::oid UNION SELECT c.conrelid FROM pg_constraint c JOIN affected a ON c.confrelid=a.oid
 WHERE c.contype='f' AND c.confdeltype IN('c','n','d'))
 SELECT c.relname,EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 AND (a.attname LIKE '%cipher%' OR a.attname LIKE '%secret%' OR a.attname LIKE '%token%' OR a.attname LIKE '%password%'))
 FROM affected a JOIN pg_class c ON c.oid=a.oid WHERE c.relname<>$2`, "public."+schema.Name, schema.Name)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		var secret bool
		if err = rows.Scan(&table, &secret); err != nil {
			return err
		}
		if tableProtection(table) != "" || secret {
			schema.AllowDelete = false
		}
	}
	return rows.Err()
}
