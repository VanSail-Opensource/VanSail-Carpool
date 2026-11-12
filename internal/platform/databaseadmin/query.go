package databaseadmin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

var filterOperators = map[string]struct{}{
	"eq": {}, "ne": {}, "contains": {}, "starts_with": {}, "gt": {}, "gte": {}, "lt": {}, "lte": {}, "is_null": {}, "not_null": {},
}

type queryCursor struct {
	Fingerprint string   `json:"fingerprint"`
	Keys        []string `json:"keys"`
}

// QueryRecords executes schema-allowlisted SQL with bound values. The sealed
// cursor carries a query fingerprint so it cannot be replayed against changed
// search text or filters.

func (s *Service) QueryRecords(ctx context.Context, table string, request QueryRequest) (Page, error) {
	schema, err := s.schema(ctx, table)
	if err != nil {
		return Page{}, err
	}
	request.Search = strings.TrimSpace(request.Search)
	if len(request.Search) > 200 || len(request.Filters) > 5 {
		return Page{}, fmt.Errorf("%w: query limits exceeded", ErrInvalidValue)
	}
	if request.Limit <= 0 || request.Limit > 200 {
		request.Limit = 50
	}
	fingerprint, err := queryFingerprint(schema.Name, request)
	if err != nil {
		return Page{}, err
	}
	conditions := make([]string, 0, len(request.Filters)+2)
	args := make([]any, 0, len(request.Filters)+8)
	if request.Search != "" {
		searchable := make([]string, 0)
		for _, column := range schema.ColumnsRaw {
			if column.Sensitive || column.Family == storageFamilyBlob || column.Hidden != 0 {
				continue
			}
			searchable = append(searchable, `CAST(`+quoteIdentifier(column.Name)+fmt.Sprintf(` AS TEXT) ILIKE $%d ESCAPE '\'`, len(args)+1))
			args = append(args, "%"+escapeLike(request.Search)+"%")
		}
		if len(searchable) > 0 {
			conditions = append(conditions, "("+strings.Join(searchable, " OR ")+")")
		}
	}
	columns := make(map[string]schemaColumn, len(schema.ColumnsRaw))
	for _, column := range schema.ColumnsRaw {
		columns[column.Name] = column
	}
	for _, filter := range request.Filters {
		column, exists := columns[filter.Column]
		if !exists {
			return Page{}, fmt.Errorf("%w: unknown filter column", ErrInvalidValue)
		}
		if _, exists := filterOperators[filter.Operator]; !exists {
			return Page{}, fmt.Errorf("%w: unknown filter operator", ErrInvalidValue)
		}
		quoted := quoteIdentifier(column.Name)
		switch filter.Operator {
		case "is_null":
			if filter.Value != nil {
				return Page{}, fmt.Errorf("%w: is_null does not accept a value", ErrInvalidValue)
			}
			conditions = append(conditions, quoted+" IS NULL")
		case "not_null":
			if filter.Value != nil {
				return Page{}, fmt.Errorf("%w: not_null does not accept a value", ErrInvalidValue)
			}
			conditions = append(conditions, quoted+" IS NOT NULL")
		default:
			if filter.Value == nil || column.Sensitive || column.Family == storageFamilyBlob {
				return Page{}, fmt.Errorf("%w: filter value is unavailable", ErrInvalidValue)
			}
			value, parseErr := parseEditorValue(column, *filter.Value)
			if parseErr != nil || value == nil {
				return Page{}, fmt.Errorf("%w: invalid filter value", ErrInvalidValue)
			}
			switch filter.Operator {
			case "contains", "starts_with":
				if column.Family != storageFamilyText || column.Boolean {
					return Page{}, fmt.Errorf("%w: text operator requires a text column", ErrInvalidValue)
				}
				text, _ := filter.Value.Text()
				pattern := "%" + escapeLike(text) + "%"
				if filter.Operator == "starts_with" {
					pattern = escapeLike(text) + "%"
				}
				conditions = append(conditions, quoted+fmt.Sprintf(" ILIKE $%d ESCAPE '\\'", len(args)+1))
				args = append(args, pattern)
			default:
				operatorSQL := map[string]string{"eq": "=", "ne": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[filter.Operator]
				conditions = append(conditions, quoted+" "+operatorSQL+fmt.Sprintf(" $%d", len(args)+1))
				args = append(args, value)
			}
		}
	}
	if strings.TrimSpace(request.Cursor) != "" {
		keys, decodeErr := s.decodeQueryCursor(schema, request.Cursor, fingerprint)
		if decodeErr != nil {
			return Page{}, fmt.Errorf("%w: invalid query cursor", ErrInvalidValue)
		}
		conditions = append(conditions, keyTuple(schema)+" > ("+placeholders(len(keys), len(args)+1)+")")
		args = append(args, keys...)
	}
	query := `SELECT ` + selectList(schema) + ` FROM ` + quoteIdentifier(schema.Name)
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}
	query += ` ORDER BY ` + keyOrder(schema) + fmt.Sprintf(" LIMIT $%d", len(args)+1)

	args = append(args, request.Limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("query %s records: %w", schema.Name, err)
	}
	defer func() { _ = rows.Close() }()
	data := make([]rowData, 0, request.Limit+1)
	for rows.Next() {
		item, scanErr := s.scanRecord(schema, rows)
		if scanErr != nil {
			return Page{}, scanErr
		}
		data = append(data, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	visible := min(request.Limit, len(data))
	page := Page{Items: make([]Record, 0, visible)}
	for index := 0; index < visible; index++ {
		page.Items = append(page.Items, data[index].record)
	}
	if len(data) > request.Limit && visible > 0 {
		next, encodeErr := s.encodeQueryCursor(schema, data[visible-1].keyRaw, fingerprint)
		if encodeErr != nil {
			return Page{}, encodeErr
		}
		page.NextCursor = &next
	}
	return page, nil
}

func queryFingerprint(table string, request QueryRequest) (string, error) {
	payload := struct {
		Table   string        `json:"table"`
		Search  string        `json:"search"`
		Filters []QueryFilter `json:"filters"`
	}{Table: table, Search: request.Search, Filters: request.Filters}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
