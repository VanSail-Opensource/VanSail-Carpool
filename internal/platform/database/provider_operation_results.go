package database

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// mergeProviderResult applies RFC 7396 to a locked operation's result. Raw
// messages preserve exact integer payloads without passing through float64.
func mergeProviderResult(document, patch string) (string, error) {
	merged, err := mergeResultJSON(json.RawMessage(document), json.RawMessage(patch))
	if err != nil {
		return "", fmt.Errorf("merge provider result: %w", err)
	}
	return string(merged), nil
}

func mergeResultJSON(document, patch json.RawMessage) (json.RawMessage, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(patch), []byte("{")) {
		return patch, nil
	}
	var changes map[string]json.RawMessage
	if err := json.Unmarshal(patch, &changes); err != nil {
		return nil, err
	}
	current := make(map[string]json.RawMessage)
	if bytes.HasPrefix(bytes.TrimSpace(document), []byte("{")) {
		if err := json.Unmarshal(document, &current); err != nil {
			return nil, err
		}
	}
	for key, value := range changes {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			delete(current, key)
			continue
		}
		merged, err := mergeResultJSON(current[key], value)
		if err != nil {
			return nil, err
		}
		current[key] = merged
	}
	return json.Marshal(current)
}
