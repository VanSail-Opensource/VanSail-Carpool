package databaseadmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Service) prepare(ctx context.Context, querier queryRower, schema tableSchema, request MutationRequest) (preparedMutation, error) {
	prepared := preparedMutation{schema: schema}
	if err := allowMutation(schema, request.Action); err != nil {
		return prepared, err
	}
	switch request.Action {
	case "update", "delete":
		current, err := s.readRecord(ctx, querier, schema, request.Key)
		if err != nil {
			return preparedMutation{}, err
		}
		if request.ExpectedHash == "" || current.record.RecordHash != request.ExpectedHash {
			return preparedMutation{}, ErrOptimisticConflict
		}
		prepared.current = &current
	case "insert":
		if len(request.Key) != 0 || request.ExpectedHash != "" {
			return preparedMutation{}, fmt.Errorf("%w: insert must not contain an existing key or record hash", ErrInvalidValue)
		}
	}
	if request.Action == "delete" {
		if len(request.Values) != 0 {
			return preparedMutation{}, fmt.Errorf("%w: delete must not contain values", ErrInvalidValue)
		}
		for _, column := range schema.ColumnsRaw {
			prepared.changedColumns = append(prepared.changedColumns, column.Name)
		}
		return prepared, nil
	}
	columns, arguments, afterRaw, changed, err := s.prepareValues(schema, prepared.current, request.Values, request.Action == "insert")
	if err != nil {
		return preparedMutation{}, err
	}
	prepared.columns, prepared.arguments, prepared.afterRaw, prepared.changedColumns = columns, arguments, afterRaw, changed
	return prepared, nil
}

func (s *Service) readRecord(ctx context.Context, querier queryRower, schema tableSchema, key map[string]string) (rowData, error) {
	keyValues, err := s.decodePublicKey(schema, key)
	if err != nil {
		return rowData{}, err
	}
	query := `SELECT ` + selectList(schema) + ` FROM ` + quoteIdentifier(schema.Name) + ` WHERE ` + keyTuple(schema) + `=(` + placeholders(len(keyValues)) + `) LIMIT 1`
	if _, transaction := querier.(*sql.Tx); transaction {
		query += ` FOR UPDATE`
	}
	data, err := s.scanRecord(schema, querier.QueryRowContext(ctx, query, keyValues...))
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(errString(err), "sql: no rows") {
		return rowData{}, ErrRecordNotFound
	}
	return data, err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Service) prepareValues(schema tableSchema, current *rowData, requested map[string]Value, insert bool) ([]schemaColumn, []any, []any, []string, error) {
	if len(requested) == 0 && !insert {
		return nil, nil, nil, nil, fmt.Errorf("%w: update contains no values", ErrInvalidValue)
	}
	byName := make(map[string]schemaColumn, len(schema.ColumnsRaw))
	for _, column := range schema.ColumnsRaw {
		byName[column.Name] = column
	}
	parsed := make(map[string]any, len(requested))
	for name, value := range requested {
		if !identifierPattern.MatchString(name) {
			return nil, nil, nil, nil, fmt.Errorf("%w: unsafe column name", ErrInvalidValue)
		}
		column, ok := byName[name]
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("%w: unknown column %s", ErrInvalidValue, name)
		}
		if reason := columnProtection(schema.Name, column, insert); reason != "" {
			return nil, nil, nil, nil, &ProtectionError{Table: schema.Name, Column: name, Reason: reason}
		}
		parsedValue, err := parseEditorValue(column, value)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		parsed[name] = parsedValue
	}
	if insert {
		for _, column := range schema.ColumnsRaw {
			if (column.Name == "created_at" || column.Name == "updated_at") && !column.Default.Valid {
				parsed[column.Name] = s.now().UTC().Format(time.RFC3339Nano)
			}
		}
	}
	afterRaw := make([]any, len(schema.ColumnsRaw))
	if current != nil {
		copy(afterRaw, current.raw)
	}
	if insert {
		for index, column := range schema.ColumnsRaw {
			if value, ok := parsed[column.Name]; ok {
				afterRaw[index] = value
				continue
			}
			if column.Hidden != 0 || column.Default.Valid || column.Nullable {
				continue
			}
			return nil, nil, nil, nil, fmt.Errorf("%w: required column %s is missing", ErrInvalidValue, column.Name)
		}
	} else {
		for index, column := range schema.ColumnsRaw {
			if value, ok := parsed[column.Name]; ok {
				afterRaw[index] = value
			}
		}
	}
	columns := make([]schemaColumn, 0, len(parsed))
	arguments := make([]any, 0, len(parsed))
	changed := make([]string, 0, len(parsed))
	for index, column := range schema.ColumnsRaw {
		value, supplied := parsed[column.Name]
		if !supplied {
			continue
		}
		columns = append(columns, column)
		arguments = append(arguments, value)
		if insert || current == nil || canonicalRaw(current.raw[index]) != canonicalRaw(afterRaw[index]) || column.Sensitive {
			changed = append(changed, column.Name)
		}
	}
	if len(changed) == 0 {
		if insert && len(parsed) == 0 {
			return columns, arguments, afterRaw, changed, nil
		}
		return nil, nil, nil, nil, fmt.Errorf("%w: mutation does not change any values", ErrInvalidValue)
	}
	sort.Strings(changed)
	return columns, arguments, afterRaw, changed, nil
}
