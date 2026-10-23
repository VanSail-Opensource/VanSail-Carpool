package subscription

import (
	"bytes"
	"encoding/json"
	"errors"
)

func decodeTuningRow(raw []byte, keys []string, target any) error {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) != len(keys) {
		return errors.New("invalid client option row")
	}
	for _, key := range keys {
		if values[key] == nil || bytes.Equal(bytes.TrimSpace(values[key]), []byte("null")) {
			return errors.New("missing client option row field")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (h *TuningHeader) UnmarshalJSON(raw []byte) error {
	type plain TuningHeader
	return decodeTuningRow(raw, []string{"name", "value"}, (*plain)(h))
}
