package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const maxJSONDepth = 64

// JSONDocument validates every token before typed decoding, without first
// materializing an unbounded tree or dropping repeated object fields.
// Printable passwords, Telegram initData, and multiline or CSV text remain valid.
func JSONDocument(document []byte) error {
	if !utf8.Valid(document) {
		return &FieldError{Field: "JSON document", Err: ErrInvalidUTF8}
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	if err := validateJSONToken(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON document contains multiple values or trailing data")
	}
	return nil
}

func validateJSONToken(decoder *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return &FieldError{Field: "JSON document", Err: ErrTooLong}
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode JSON validation document: %w", err)
	}
	switch value := token.(type) {
	case string:
		return Text("JSON string", value, -1, true)
	case json.Delim:
		switch value {
		case '{':
			keys := make(map[string]struct{})
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return fmt.Errorf("decode JSON object key: %w", err)
				}
				key, ok := token.(string)
				if !ok {
					return &FieldError{Field: "JSON object key", Err: ErrPattern}
				}
				if err := Text("JSON object key", key, 256, false); err != nil {
					return err
				}
				if _, repeated := keys[key]; repeated {
					return &FieldError{Field: "duplicate JSON object key", Err: ErrPattern}
				}
				keys[key] = struct{}{}
				if err := validateJSONToken(decoder, depth+1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := validateJSONToken(decoder, depth+1); err != nil {
					return err
				}
			}
		default:
			return &FieldError{Field: "JSON document", Err: ErrPattern}
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("decode JSON container: %w", err)
		}
	}
	return nil
}
