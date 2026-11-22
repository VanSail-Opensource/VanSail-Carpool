package pricing

import (
	"testing"
	"time"
)

func TestHealthPollCannotExtendTelemetryFreshness(t *testing.T) {
	start := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		validUntil time.Time
		window     time.Duration
	}{
		{name: "receipt limit", window: time.Minute},
		{name: "negotiated report window", validUntil: start.Add(15 * time.Second), window: 15 * time.Second},
		{name: "receipt limit caps longer window", validUntil: start.Add(10 * time.Minute), window: time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewHealthRegistry()
			o := Observation{NodeID: "node", At: start, ReceivedAt: start, ReportedAt: start.Add(3 * time.Minute), ValidUntil: tc.validUntil, Connected: true, Ready: true, Synchronized: true, Complete: true, CPU: 20, Memory: 20}
			r.Observe(o)
			o.At = start.Add(tc.window - time.Second)
			if h := r.Observe(o); h.State != "HEALTHY" {
				t.Fatalf("fresh repeated observation: %+v", h)
			}
			deadline := start.Add(tc.window)
			if !r.Safe([]string{"node"}, deadline) {
				t.Fatal("exact freshness boundary changed")
			}
			expired := deadline.Add(time.Nanosecond)
			if r.Safe([]string{"node"}, expired) || r.TelemetrySafe([]string{"node"}, expired) {
				t.Fatal("poll time extended old telemetry admission")
			}
			if snapshot := r.Snapshot(expired); snapshot[0].State != "TELEMETRY_STALE" {
				t.Fatalf("expired telemetry still visible as healthy: %+v", snapshot)
			}
			o.At = expired
			if h := r.Observe(o); h.State != "TELEMETRY_STALE" {
				t.Fatalf("observer ignored receipt deadline: %+v", h)
			}
		})
	}
}

func TestHealthSnapshotPreservesFreshIndependentOutageEvidence(t *testing.T) {
	start := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, state := range []string{"OFFLINE", "MAINTENANCE"} {
		t.Run(state, func(t *testing.T) {
			r := NewHealthRegistry()
			o := Observation{NodeID: "node", At: start, Stale: true, Maintenance: state == "MAINTENANCE"}
			r.Observe(o)
			o.At = start.Add(time.Minute)
			r.Observe(o)
			now := o.At.Add(5 * time.Second)
			if h := r.Snapshot(now)[0]; h.State != state {
				t.Fatalf("missing telemetry hid independent evidence: %+v", h)
			}
			if r.Safe([]string{"node"}, now) || r.TelemetrySafe([]string{"node"}, now) {
				t.Fatal("independent outage evidence authorized work")
			}
			if h := r.Snapshot(o.At.Add(time.Minute + time.Nanosecond))[0]; h.State != "TELEMETRY_STALE" {
				t.Fatalf("stopped observer kept old outage evidence fresh: %+v", h)
			}
		})
	}
}

func TestInterruptedHealthEvidenceRestartsOverloadAndRecovery(t *testing.T) {
	r := NewHealthRegistry()
	start := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	observe := func(seconds int, cpu float64, complete bool) Health {
		at := start.Add(time.Duration(seconds) * time.Second)
		return r.Observe(Observation{NodeID: "node", At: at, ReceivedAt: at, ValidUntil: at.Add(20 * time.Second), CPU: cpu, Memory: 20, Connected: true, Ready: true, Synchronized: true, Complete: complete})
	}
	for _, second := range []int{0, 15, 30, 45} {
		observe(second, 100, true)
	}
	// Nobody polled while the report expired. The next sample must start over.
	if h := observe(70, 100, true); h.State == "OVERLOADED" {
		t.Fatal(h)
	}
	for _, second := range []int{85, 100, 115, 130} {
		observe(second, 100, true)
	}
	if r.Snapshot(start.Add(130 * time.Second))[0].State != "OVERLOADED" {
		t.Fatal("sustained fresh overload was missed")
	}
	for i := 145; i <= 415; i += 15 {
		observe(i, 20, true)
	}
	observe(430, 20, false)
	if h := observe(445, 20, true); h.State != "RECOVERING" {
		t.Fatal(h)
	}
}
