package xnode

import "testing"

func TestNewReportingTargetPreservesSavedIntervals(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		seconds int
	}{
		{"", 2}, {`{}`, 5}, {`{"telemetry":{"report_seconds":0}}`, 5},
		{`{"telemetry":{"report_seconds":12}}`, 12}, {`{"telemetry":{"report_seconds":2}}`, 2},
	} {
		config, err := DecodeGlobal(tc.raw)
		if err != nil || config.Telemetry.ReportSeconds != tc.seconds {
			t.Fatalf("saved reporting policy changed: %s => %d %v", tc.raw, config.Telemetry.ReportSeconds, err)
		}
	}
}
