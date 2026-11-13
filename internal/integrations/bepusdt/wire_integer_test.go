package bepusdt

import (
	"encoding/json"
	"testing"
)

func TestIntegerMetadataPreservesExactWireDigits(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  int64
	}{
		{input: "9223372036", want: 9_223_372_036},
		{input: "9007199254740993", want: 9_007_199_254_740_993},
		{input: "9223372036854775807", want: 1<<63 - 1},
		{input: "-9223372036854775808", want: -1 << 63},
		{input: `"9223372036854775807"`, want: 1<<63 - 1},
		{input: `"200"`, want: 200},
	} {
		var value flexibleInt
		if err := json.Unmarshal([]byte(test.input), &value); err != nil || int64(value) != test.want {
			t.Fatalf("integer %s = %d, %v; want %d", test.input, value, err, test.want)
		}
	}
	for _, input := range []string{"9223372036854775808", "-9223372036854775809", "1.5", "null", "true", "[]", "{}"} {
		var value flexibleInt
		if err := json.Unmarshal([]byte(input), &value); err == nil {
			t.Fatalf("invalid integer %s was accepted", input)
		}
	}
}
