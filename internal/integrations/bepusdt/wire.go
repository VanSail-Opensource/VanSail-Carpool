package bepusdt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	value, err := scalarString(data)
	if err != nil {
		return err
	}
	*s = flexibleString(value)
	return nil
}

type flexibleInt int64

func (i *flexibleInt) UnmarshalJSON(data []byte) error {
	// Integer metadata must retain its original decimal digits. The float64
	// canonicalization used by callback signatures can round values or emit
	// scientific notation and is not a numeric field decoder.
	trimmed := bytes.TrimSpace(data)
	var value string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
	} else {
		var number json.Number
		if err := json.Unmarshal(trimmed, &number); err != nil {
			return err
		}
		value = number.String()
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("parse integer: %w", err)
	}
	*i = flexibleInt(parsed)
	return nil
}

func scalarString(data []byte) (string, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return "", err
		}
		return value, nil
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		return providerNumberString(number.String()), nil
	}
	if bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) {
		return string(trimmed), nil
	}
	return "", errors.New("value must be a JSON scalar")
}

// providerNumberString mirrors BEPusdt's signing implementation. The
// upstream server unmarshals JSON into map[string]interface{}, which turns
// JSON numbers into float64 before formatting them with %v.
func providerNumberString(raw string) string {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return raw
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func decodeUniqueObject(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	opening, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("bepusdt decode webhook: %w", err)
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("bepusdt webhook must be a JSON object")
	}
	result := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("bepusdt decode webhook key: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("bepusdt webhook contains a non-string key")
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("bepusdt webhook contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("bepusdt decode webhook field %q: %w", key, err)
		}
		result[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("bepusdt decode webhook end: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("bepusdt webhook contains trailing JSON")
		}
		return nil, fmt.Errorf("bepusdt decode trailing data: %w", err)
	}
	return result, nil
}
