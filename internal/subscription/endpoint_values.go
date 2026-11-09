package subscription

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TuningPortRange is a compact inclusive client destination-port interval.
type TuningPortRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

func (o *EndpointOptions) UnmarshalJSON(raw []byte) error {
	if len(raw) > 128<<10 {
		return errors.New("endpoint options exceed limit")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return errors.New("endpoint options must be an object")
	}
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("null endpoint option: %s", key)
		}
	}
	type plain EndpointOptions
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*plain)(o))
}

func (o EndpointOptions) values() (map[string]any, error) {
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	err = json.Unmarshal(raw, &values)
	return values, err
}

func endpointValues(p Proxy) map[string]any {
	if p.EndpointTuning == nil {
		return map[string]any{}
	}
	values, _ := p.EndpointTuning.values()
	return values
}

func tuningPathValue(root map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := bytes.Split([]byte(path), []byte("."))
	current := root
	for _, key := range parts[:len(parts)-1] {
		current = object(current, string(key))
		if current == nil {
			return nil, false
		}
	}
	value, ok := current[string(parts[len(parts)-1])]
	return value, ok
}

func endpointMeaningful(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case []any:
		return len(v) != 0
	case map[string]any:
		return len(v) != 0
	}
	return true
}

func endpointInheritedValue(p Proxy, spec tuningSpec) (any, bool) {
	path := endpointXrayPath(spec.Mappings["xray"], p, nil)
	if path == "" || !endpointContextAvailable(spec, p, nil) {
		return nil, false
	}
	root := map[string]any{"streamSettings": p.Stream}
	if strings.HasPrefix(path, "streamSettings.xhttpSettings.extra.") && object(object(p.Stream, "xhttpSettings"), "extra") == nil {
		path = strings.Replace(path, ".xhttpSettings.extra.", ".xhttpSettings.", 1)
	}
	return tuningPathValue(root, path)
}
