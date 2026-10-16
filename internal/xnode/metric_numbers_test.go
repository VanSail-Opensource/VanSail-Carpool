package xnode

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMetricCountersPreserveExactUnsignedValuesForBrowsers(t *testing.T) {
	raw := BrowserMetricNumbers(json.RawMessage(`{"memory":{"used":9007199254740993},"interfaces":[{"sent":18446744073709551615}],"load":0.25,"count":42}`))
	for _, want := range []string{`"used":"9007199254740993"`, `"sent":"18446744073709551615"`, `"load":0.25`, `"count":42`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("metric integer lost: %s", raw)
		}
	}
}
