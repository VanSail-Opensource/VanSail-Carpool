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

func TestSavedActiveIPOptOutIsNormalized(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"telemetry":{"include_active_ips":false}}`, `{"telemetry":{"include_active_ips":true,"sample_seconds":12,"report_seconds":17}}`} {
		config, err := DecodeGlobal(raw)
		if err != nil || !config.Telemetry.IncludeActiveIPs {
			t.Fatal("saved opt-out disabled mandatory reporting", raw, err)
		}
		if raw == `{"telemetry":{"include_active_ips":true,"sample_seconds":12,"report_seconds":17}}` && (config.Telemetry.SampleSeconds != 12 || config.Telemetry.ReportSeconds != 17) {
			t.Fatal("normalization changed administrator intervals")
		}
	}
}
