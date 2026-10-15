package jsonlimit

import (
	"bytes"
	"encoding/json"
	"io"
)

// DecodeSlice rejects excess items before decoding or appending the next item.
func DecodeSlice[T any](raw []byte, limit int) ([]T, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
		return nil, ErrLimit
	}
	var result []T
	for decoder.More() {
		if len(result) >= limit {
			return nil, ErrLimit
		}
		var item T
		if err := decoder.Decode(&item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrLimit
	}
	return result, nil
}
