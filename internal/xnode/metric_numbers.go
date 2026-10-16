package xnode

import (
	"bytes"
	"encoding/json"
	"math/big"
)

// BrowserMetricNumbers ensures metric integers outside JavaScript's exact range cross the HTTP boundary as
// decimal strings. The agent wire format and memory-only snapshots stay intact.
func BrowserMetricNumbers(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return json.RawMessage(`{}`)
	}
	var walk func(any) any
	maximum := big.NewInt(9007199254740991)
	walk = func(value any) any {
		switch item := value.(type) {
		case json.Number:
			if integer, ok := new(big.Int).SetString(item.String(), 10); ok && new(big.Int).Abs(integer).Cmp(maximum) > 0 {
				return item.String()
			}
		case []any:
			for i, entry := range item {
				item[i] = walk(entry)
			}
		case map[string]any:
			for key, entry := range item {
				item[key] = walk(entry)
			}
		}
		return value
	}
	encoded, err := json.Marshal(walk(value))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
