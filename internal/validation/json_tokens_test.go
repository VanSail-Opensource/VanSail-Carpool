package validation

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestJSONDocumentValidatesEveryObjectBeforeTypedMerge(t *testing.T) {
	// encoding/json merges repeated nested objects into typed destinations.
	// A map-first validator used to discard the earlier object's invalid field.
	raw := []byte(`{"profile":{"name":"\u0000"},"profile":{"enabled":true}}`)
	var target struct {
		Profile struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(raw, &target); err != nil || target.Profile.Name != "\x00" || !target.Profile.Enabled {
		t.Fatal("fixture did not reproduce typed-object merge", target, err)
	}
	if err := JSONDocument(raw); !errors.Is(err, ErrControlCharacter) {
		t.Fatal("overwritten object hid an invalid string", err)
	}
	// Different casing is significant for arbitrary JSON maps. Both values must
	// still be validated when Go later treats them as aliases for a typed field.
	if err := JSONDocument([]byte(`{"profile":{"name":"\u0000"},"Profile":{"enabled":true}}`)); !errors.Is(err, ErrControlCharacter) {
		t.Fatal("case-insensitive typed alias hid an invalid string", err)
	}
}

func TestJSONDocumentRejectsAmbiguousKeysAndBoundsNestedInput(t *testing.T) {
	for _, raw := range []string{
		`{"amount":1,"amount":2}`,
		`{"amount":1,"\u0061mount":2}`,
		`{"outer":{"field":1,"field":2}}`,
	} {
		if err := JSONDocument([]byte(raw)); !errors.Is(err, ErrPattern) {
			t.Errorf("duplicate object key accepted: %s (%v)", raw, err)
		}
	}
	for _, raw := range []string{
		`{"first":{"field":1},"second":{"field":2}}`,
		`{"header":{"X-Custom":"a","x-custom":"b"}}`,
		`{"number":9007199254740993,"text":"中文\ntext"}`,
		strings.Repeat("[", maxJSONDepth) + "0" + strings.Repeat("]", maxJSONDepth),
	} {
		if err := JSONDocument([]byte(raw)); err != nil {
			t.Errorf("valid JSON rejected: %s (%v)", raw, err)
		}
	}
	tooDeep := strings.Repeat("[", maxJSONDepth+1) + "0" + strings.Repeat("]", maxJSONDepth+1)
	if err := JSONDocument([]byte(tooDeep)); !errors.Is(err, ErrTooLong) {
		t.Fatal("excessive nesting was not rejected at the token boundary", err)
	}
	for _, raw := range []string{`{"field":1`, `[1,]`, `{"field":1} null`, `{"field":1} trailing`} {
		if err := JSONDocument([]byte(raw)); err == nil {
			t.Errorf("malformed document accepted: %s", raw)
		}
	}
}
