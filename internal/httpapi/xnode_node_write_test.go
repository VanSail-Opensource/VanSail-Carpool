package httpapi

import (
	"encoding/json"
	"testing"
)

func TestNodeWriteRequiresAnExplicitBoundedSixDecimalMultiplier(t *testing.T) {
	for _, test := range []struct {
		input string
		valid bool
	}{
		{`{}`, false}, {`{"multiplier":null}`, false}, {`{"multiplier":"1"}`, false},
		{`{"multiplier":0}`, true}, {`{"multiplier":0.000001}`, true},
		{`{"multiplier":1.123456}`, true}, {`{"multiplier":1000.000000}`, true},
		{`{"multiplier":-0.000001}`, false}, {`{"multiplier":1000.000001}`, false},
		{`{"multiplier":1.0000000}`, false}, {`{"multiplier":0.0000001}`, false},
		{`{"multiplier":1e-7}`, false},
	} {
		t.Run(test.input, func(t *testing.T) {
			var input nodeWrite
			if err := json.Unmarshal([]byte(test.input), &input); err != nil {
				t.Fatal(err)
			}
			_, err := explicitNodeMultiplier(input.Multiplier)
			if (err == nil) != test.valid {
				t.Fatal("multiplier presence/precision boundary", test.input, err)
			}
		})
	}
}
