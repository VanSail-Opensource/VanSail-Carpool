package subscription

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type TuningRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}
type TuningHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type TuningSocketOption struct {
	System  string `json:"system"`
	Network string `json:"network"`
	Level   string `json:"level"`
	Opt     string `json:"opt"`
	Value   string `json:"value"`
	Type    string `json:"type"`
}

// FastOpenSetting is the pinned socket parser's bounded boolean/integer union.
// It accepts no object, array, fractional number or raw JSON extension.
type FastOpenSetting struct {
	boolean *bool
	count   *int32
}

func (f *FastOpenSetting) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(raw, []byte("true")) || bytes.Equal(raw, []byte("false")) {
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		f.boolean = &value
		f.count = nil
		return nil
	}
	var value int32
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	f.count = &value
	f.boolean = nil
	return nil
}
func (f FastOpenSetting) MarshalJSON() ([]byte, error) {
	if f.boolean != nil {
		return json.Marshal(*f.boolean)
	}
	if f.count != nil {
		return json.Marshal(*f.count)
	}
	return nil, errors.New("missing TCP Fast Open value")
}

func (o *ConnectionOverrides) UnmarshalJSON(raw []byte) error {
	if len(raw) > 64<<10 {
		return errors.New("client tuning exceeds limit")
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return errors.New("client tuning must be an object")
	}
	for key, value := range values {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("null client option: %s", key)
		}
	}
	type plain ConnectionOverrides
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*plain)(o))
}

func (o ConnectionOverrides) values() (map[string]any, error) {
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	if err = json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (r *TuningRange) UnmarshalJSON(raw []byte) error {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) != 2 || values["min"] == nil || values["max"] == nil {
		return errors.New("range requires minimum and maximum")
	}
	for _, value := range values {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("range values cannot be null")
		}
	}
	type plain TuningRange
	return json.Unmarshal(raw, (*plain)(r))
}
