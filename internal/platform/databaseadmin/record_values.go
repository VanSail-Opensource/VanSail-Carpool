package databaseadmin

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func publicValue(column schemaColumn, raw any) (Value, error) {
	if raw == nil {
		return NullValue(), nil
	}
	if column.Boolean {
		switch value := raw.(type) {
		case int64:
			if value == 0 || value == 1 {
				return BooleanValue(value == 1), nil
			}
		case bool:
			return BooleanValue(value), nil
		}
	}
	switch value := raw.(type) {
	case time.Time:
		return StringValue(value.UTC().Format(time.RFC3339Nano)), nil
	case int64:
		return StringValue(strconv.FormatInt(value, 10)), nil
	case float64:
		return StringValue(strconv.FormatFloat(value, 'g', -1, 64)), nil
	case string:
		if column.Family == storageFamilyBlob {
			return BlobValue([]byte(value)), nil
		}
		return StringValue(value), nil
	case []byte:
		if column.Family == storageFamilyBlob {
			return BlobValue(value), nil
		}
		return StringValue(string(value)), nil
	case bool:
		return BooleanValue(value), nil
	default:
		return Value{}, fmt.Errorf("unsupported PostgreSQL value %T", raw)
	}
}

func recordHash(schema tableSchema, raw, keyRaw []any) (string, error) {
	payload := struct {
		Table   string   `json:"table"`
		Columns []string `json:"columns"`
		Values  []string `json:"values"`
		Keys    []string `json:"keys"`
	}{Table: schema.Name}
	for index, column := range schema.ColumnsRaw {
		payload.Columns = append(payload.Columns, column.Name)
		payload.Values = append(payload.Values, canonicalRaw(raw[index]))
	}
	for _, value := range keyRaw {
		payload.Keys = append(payload.Keys, canonicalRaw(value))
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode record hash: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalRaw(value any) string {
	switch typed := value.(type) {
	case time.Time:
		return "timestamp:" + typed.UTC().Format(time.RFC3339Nano)
	case nil:
		return "null:"
	case int64:
		return "integer:" + strconv.FormatInt(typed, 10)
	case float64:
		return "real:" + strconv.FormatFloat(typed, 'g', -1, 64)
	case []byte:
		return "blob:" + base64.StdEncoding.EncodeToString(typed)
	case string:
		return "text:" + typed
	case bool:
		return "boolean:" + strconv.FormatBool(typed)
	default:
		return fmt.Sprintf("%T:%v", value, value)
	}
}

func canonicalKeyValue(column schemaColumn, raw any) (string, error) {
	if raw == nil {
		return "", fmt.Errorf("%w: primary key %s is NULL", ErrInvalidValue, column.Name)
	}
	switch value := raw.(type) {
	case time.Time:
		return value.UTC().Format(time.RFC3339Nano), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64), nil
	case string:
		return value, nil
	case []byte:
		return base64.StdEncoding.EncodeToString(value), nil
	default:
		return "", fmt.Errorf("%w: unsupported primary key type %T", ErrInvalidValue, raw)
	}
}
