package databaseadmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type rowData struct {
	raw    []any
	keyRaw []any
	record Record
}

// Records returns a cursor page ordered by the declared primary key, or rowid
// only when the table has no declared key. Browser-supplied table names are
// resolved through schema introspection before any identifier is composed.
func (s *Service) Records(ctx context.Context, table, cursor string, limit int) (Page, error) {
	schema, err := s.schema(ctx, table)
	if err != nil {
		return Page{}, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	selectSQL := selectList(schema)
	query := `SELECT ` + selectSQL + ` FROM ` + quoteIdentifier(schema.Name)
	args := make([]any, 0)
	if strings.TrimSpace(cursor) != "" {
		keyValues, err := s.decodeCursor(schema, cursor)
		if err != nil {
			return Page{}, fmt.Errorf("%w: invalid record cursor", ErrInvalidValue)
		}
		query += ` WHERE ` + keyTuple(schema) + ` > (` + placeholders(len(keyValues)) + `)`
		args = append(args, keyValues...)
	}
	query += ` ORDER BY ` + keyOrder(schema) + fmt.Sprintf(" LIMIT $%d", len(args)+1)

	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("list %s records: %w", schema.Name, err)
	}
	defer rows.Close()
	data := make([]rowData, 0, limit+1)
	for rows.Next() {
		item, err := s.scanRecord(schema, rows)
		if err != nil {
			return Page{}, err
		}
		data = append(data, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate %s records: %w", schema.Name, err)
	}
	page := Page{Items: make([]Record, 0, min(limit, len(data)))}
	visible := len(data)
	if visible > limit {
		visible = limit
	}
	for index := 0; index < visible; index++ {
		page.Items = append(page.Items, data[index].record)
	}
	if len(data) > limit && visible > 0 {
		next, err := s.encodeCursor(schema, data[visible-1].keyRaw)
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = &next
	}
	return page, nil
}

func selectList(schema tableSchema) string {
	parts := make([]string, 0, len(schema.ColumnsRaw)+1)
	for _, column := range schema.ColumnsRaw {
		parts = append(parts, quoteIdentifier(column.Name))
	}

	return strings.Join(parts, `,`)
}

func keyTuple(schema tableSchema) string {

	parts := make([]string, 0, len(schema.KeyColumns))
	for _, column := range schema.KeyColumns {
		parts = append(parts, quoteIdentifier(column.Name))
	}
	return `(` + strings.Join(parts, `,`) + `)`
}

func keyOrder(schema tableSchema) string {

	parts := make([]string, 0, len(schema.KeyColumns))
	for _, column := range schema.KeyColumns {
		parts = append(parts, quoteIdentifier(column.Name)+` ASC`)
	}
	return strings.Join(parts, `,`)
}

func placeholders(count int, first ...int) string {
	start := 1
	if len(first) > 0 {
		start = first[0]
	}
	values := make([]string, count)
	for i := range values {
		values[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(values, ",")
}

func (s *Service) scanRecord(schema tableSchema, row interface{ Scan(...any) error }) (rowData, error) {
	values := make([]any, len(schema.ColumnsRaw))
	destinations := make([]any, len(values))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := row.Scan(destinations...); err != nil {
		return rowData{}, fmt.Errorf("scan %s record: %w", schema.Name, err)
	}
	keyRaw := make([]any, 0, max(1, len(schema.KeyColumns)))

	for _, key := range schema.KeyColumns {
		for index, column := range schema.ColumnsRaw {
			if column.Name == key.Name {
				keyRaw = append(keyRaw, values[index])
				break
			}
		}
	}

	record, err := s.makeRecord(schema, values[:len(schema.ColumnsRaw)], keyRaw)
	if err != nil {
		return rowData{}, err
	}
	return rowData{raw: values[:len(schema.ColumnsRaw)], keyRaw: keyRaw, record: record}, nil
}

func (s *Service) makeRecord(schema tableSchema, raw, keyRaw []any) (Record, error) {
	record := Record{Key: make(map[string]string), Values: make(map[string]Value, len(schema.ColumnsRaw))}
	for index, column := range schema.ColumnsRaw {
		if column.Sensitive {
			record.Values[column.Name] = maskedValue()
			continue
		}
		value, err := publicValue(column, raw[index])
		if err != nil {
			return Record{}, fmt.Errorf("render %s.%s: %w", schema.Name, column.Name, err)
		}
		record.Values[column.Name] = value
	}

	for index, column := range schema.KeyColumns {
		encoded, err := canonicalKeyValue(column, keyRaw[index])
		if err != nil {
			return Record{}, err
		}
		if column.Sensitive {
			if s.vault == nil {
				return Record{}, errors.New("vault is required to conceal a sensitive record key")
			}
			sealed, err := s.vault.Encrypt(keyContext(schema.Name, column.Name), encoded)
			if err != nil {
				return Record{}, fmt.Errorf("seal sensitive record key: %w", err)
			}
			encoded = "sealed:" + sealed
		}
		record.Key[column.Name] = encoded
	}

	hash, err := recordHash(schema, raw, keyRaw)
	if err != nil {
		return Record{}, err
	}
	record.RecordHash = hash
	return record, nil
}
