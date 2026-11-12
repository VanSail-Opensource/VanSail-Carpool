package providerops

import (
	"strings"
	"testing"
)

func TestResultObjectPreservesExactNestedNumbers(t *testing.T) {
	input := `{"sent":9007199254740993,"nested":{"spent":9223372036854775807,"refund":-9223372036854775808},"fraction":0.123456789012345678901234}`
	want := `{"fraction":0.123456789012345678901234,"nested":{"refund":-9223372036854775808,"spent":9223372036854775807},"sent":9007199254740993}`
	got, err := ResultObject(input)
	if err != nil || got != want {
		t.Fatalf("durable result lost precision: %s, %v", got, err)
	}
	replayed, err := ResultObject(got)
	if err != nil || replayed != got {
		t.Fatalf("canonical result changed on replay: %s, %v", replayed, err)
	}
}

func TestResultObjectRequiresOneBoundedObject(t *testing.T) {
	for _, input := range []string{"null", "[]", "true", "1", "{} {}", "{} trailing", "{", `{"value":"` + strings.Repeat("x", 16*1024) + `"}`} {
		if _, err := ResultObject(input); err == nil {
			t.Fatalf("invalid durable result accepted: %.40s", input)
		}
	}
	if got, err := ResultObject("  "); err != nil || got != "{}" {
		t.Fatalf("empty metadata compatibility changed: %s, %v", got, err)
	}
}
