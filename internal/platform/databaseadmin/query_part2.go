package databaseadmin

import (
	"encoding/json"
	"errors"

	"strings"
)

func (s *Service) encodeQueryCursor(schema tableSchema, raw []any, fingerprint string) (string, error) {
	if s.vault == nil {
		return "", errors.New("vault is required for query cursors")
	}
	keys := make([]string, 0, len(raw))

	for index, column := range schema.KeyColumns {
		value, err := canonicalKeyValue(column, raw[index])
		if err != nil {
			return "", err
		}
		keys = append(keys, value)
	}

	encoded, err := json.Marshal(queryCursor{Fingerprint: fingerprint, Keys: keys})
	if err != nil {
		return "", err
	}
	sealed, err := s.vault.Encrypt("databaseadmin:query-cursor:"+schema.Name, string(encoded))
	if err != nil {
		return "", err
	}
	return "sealed:" + sealed, nil
}

func (s *Service) decodeQueryCursor(schema tableSchema, cursor, fingerprint string) ([]any, error) {
	if s.vault == nil || !strings.HasPrefix(cursor, "sealed:") {
		return nil, errors.New("invalid cursor format")
	}
	plain, err := s.vault.Decrypt("databaseadmin:query-cursor:"+schema.Name, strings.TrimPrefix(cursor, "sealed:"))
	if err != nil {
		return nil, err
	}
	var decoded queryCursor
	if err := json.Unmarshal([]byte(plain), &decoded); err != nil || decoded.Fingerprint != fingerprint {
		return nil, errors.New("query cursor fingerprint differs")
	}

	if len(decoded.Keys) != len(schema.KeyColumns) {
		return nil, errors.New("query cursor key shape differs")
	}
	values := make([]any, 0, len(decoded.Keys))
	for index, column := range schema.KeyColumns {
		value, err := parseKeyValue(column, decoded.Keys[index])
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
