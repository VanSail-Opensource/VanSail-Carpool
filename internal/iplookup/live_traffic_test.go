package iplookup

import (
	"context"
	"net/http"
	"testing"
)

func TestTrafficSummaryPreservesUnavailableCoverageForRepair(t *testing.T) {
	for _, tc := range []struct {
		name, result, status string
		values               int
		partial              bool
	}{
		{"invalid", `{"summary_0":{"bot":"NaN","human":"101"}}`, "partial", 0, true},
		{"mixed", `{"summary_0":{"bot":"-1","human":"80"}}`, "partial", 1, true},
		{"missing", `{}`, "error", 0, true},
		{"null", `{"summary_0":null}`, "error", 0, true},
		{"wrong type", `{"summary_0":[]}`, "error", 0, true},
		{"empty", `{"summary_0":{}}`, "empty", 0, false},
		{"zero", `{"summary_0":{"bot":"0","human":"100"}}`, "success", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := liveTestService(t, CloudflareID, func(*http.Request) (int, string) {
				return 200, `{"success":true,"result":` + tc.result + `}`
			})
			summary := s.trafficSummary(context.Background(), 13335, "BOT_CLASS", "fixture-secret")
			if summary.Status != tc.status || len(summary.Values) != tc.values {
				t.Fatalf("summary = %+v", summary)
			}
			if tc.name == "mixed" && summary.Values["human"] != 80 {
				t.Fatal("valid observation lost with a malformed sibling")
			}
			details := LiveDetails{ASNs: []ASNDetails{{ASN: 13335, Traffic: ASNTraffic{BotHuman: summary}}}}
			if LivePartial(details) != tc.partial || liveRetryPlanOf(details).ASNs != tc.partial {
				t.Fatal("traffic availability did not reach snapshot status and repair planning")
			}
		})
	}
}
