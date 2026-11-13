package xnode

import (
	"encoding/json"
	"strings"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

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

func TestSavedInertActiveIPFieldIsRetiredWithoutChangingIntervals(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"telemetry":{"include_active_ips":false}}`, `{"telemetry":{"include_active_ips":true,"sample_seconds":12,"report_seconds":17}}`} {
		config, err := DecodeGlobal(raw)
		if err != nil {
			t.Fatal("saved retired reporting input broke current configuration", raw, err)
		}
		canonical, err := json.Marshal(config)
		if err != nil || strings.Contains(string(canonical), "include_active_ips") {
			t.Fatal("retired saved field escaped into current output", err)
		}
		if raw == `{"telemetry":{"include_active_ips":true,"sample_seconds":12,"report_seconds":17}}` && (config.Telemetry.SampleSeconds != 12 || config.Telemetry.ReportSeconds != 17) {
			t.Fatal("normalization changed administrator intervals")
		}
	}
}

func TestGlobalRetirementDoesNotPermitRetiredLiveOrUnknownSavedFields(t *testing.T) {
	for _, raw := range []string{`{"telemetry":{"include_active_ips":false}}`, `{"telemetry":{"include_active_ips":true}}`} {
		var current GlobalConfig
		if err := wire.Decode([]byte(raw), &current); err == nil {
			t.Fatal("retired telemetry option accepted as a current write", raw)
		}
	}
	for _, raw := range []string{`{"telemetry":{"include_active_ips":"false"}}`, `{"telemetry":{"include_active_ips":false,"unknown":1}}`, `{"telemetry":[]}`} {
		if _, err := DecodeGlobal(raw); err == nil {
			t.Fatal("retired-field migration hid invalid saved input", raw)
		}
	}
}
