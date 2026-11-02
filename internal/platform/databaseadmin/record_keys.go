package databaseadmin

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) decodePublicKey(schema tableSchema, key map[string]string) ([]any, error) {

	if len(key) != len(schema.KeyColumns) {
		return nil, fmt.Errorf("%w: primary key shape differs", ErrInvalidValue)
	}
	values := make([]any, 0, len(schema.KeyColumns))
	for _, column := range schema.KeyColumns {
		encoded, ok := key[column.Name]
		if !ok {
			return nil, fmt.Errorf("%w: primary key %s is required", ErrInvalidValue, column.Name)
		}
		if column.Sensitive {
			if s.vault == nil || !strings.HasPrefix(encoded, "sealed:") {
				return nil, fmt.Errorf("%w: sensitive primary key is not sealed", ErrInvalidValue)
			}
			plain, err := s.vault.Decrypt(keyContext(schema.Name, column.Name), strings.TrimPrefix(encoded, "sealed:"))
			if err != nil {
				return nil, fmt.Errorf("%w: sensitive primary key cannot be opened", ErrInvalidValue)
			}
			encoded = plain
		}
		parsed, err := parseKeyValue(column, encoded)
		if err != nil {
			return nil, err
		}
		values = append(values, parsed)
	}
	return values, nil
}

func parseKeyValue(column schemaColumn, encoded string) (any, error) {
	switch column.Family {
	case storageFamilyInteger:
		value, err := strconv.ParseInt(encoded, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: key %s must be a decimal integer", ErrInvalidValue, column.Name)
		}
		return value, nil
	case storageFamilyReal:
		value, err := strconv.ParseFloat(encoded, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: key %s must be numeric", ErrInvalidValue, column.Name)
		}
		return value, nil
	case storageFamilyNumeric:
		// Keep NUMERIC keys as validated decimal text. Binding them as float64
		// would round values above 2^53. PostgreSQL infers NUMERIC from the
		// comparison column and receives the exact validated decimal text.
		if !decimalPattern.MatchString(encoded) {
			return nil, fmt.Errorf("%w: key %s must be numeric", ErrInvalidValue, column.Name)
		}
		return encoded, nil
	case storageFamilyBlob:
		value, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("%w: key %s must be base64", ErrInvalidValue, column.Name)
		}
		return value, nil
	default:
		return encoded, nil
	}
}

func keyContext(table, column string) string { return "databaseadmin:key:" + table + ":" + column }

func (s *Service) encodeCursor(schema tableSchema, raw []any) (string, error) {
	values := make([]string, 0, len(raw))

	for index, column := range schema.KeyColumns {
		value, err := canonicalKeyValue(column, raw[index])
		if err != nil {
			return "", err
		}
		values = append(values, value)
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode record cursor: %w", err)
	}
	if s.vault != nil {
		sealed, err := s.vault.Encrypt("databaseadmin:cursor:"+schema.Name, string(encoded))
		if err != nil {
			return "", fmt.Errorf("seal record cursor: %w", err)
		}
		return "sealed:" + sealed, nil
	}
	return "plain:" + base64.RawURLEncoding.EncodeToString(encoded), nil
}

func (s *Service) decodeCursor(schema tableSchema, cursor string) ([]any, error) {
	var encoded []byte
	switch {
	case strings.HasPrefix(cursor, "sealed:") && s.vault != nil:
		plain, err := s.vault.Decrypt("databaseadmin:cursor:"+schema.Name, strings.TrimPrefix(cursor, "sealed:"))
		if err != nil {
			return nil, err
		}
		encoded = []byte(plain)
	case strings.HasPrefix(cursor, "plain:") && s.vault == nil:
		value, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(cursor, "plain:"))
		if err != nil {
			return nil, err
		}
		encoded = value
	default:
		return nil, errors.New("cursor format is invalid")
	}
	var values []string
	if err := json.Unmarshal(encoded, &values); err != nil {
		return nil, err
	}
	expected := len(schema.KeyColumns)

	if len(values) != expected {
		return nil, errors.New("cursor key shape differs")
	}
	parsed := make([]any, 0, expected)

	for index, column := range schema.KeyColumns {
		value, err := parseKeyValue(column, values[index])
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, value)
	}
	return parsed, nil
}
