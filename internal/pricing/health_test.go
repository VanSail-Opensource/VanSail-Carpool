package pricing

import (
	"testing"
	"time"
)

func TestHealthPersistenceStaleTelemetryAndRecovery(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	r := NewHealthRegistry()
	observe := func(at time.Time, cpu float64, stale, connected bool) Health {
		return r.Observe(Observation{NodeID: "node", At: at, ReportedAt: at, CPU: cpu, Memory: 20, Connected: connected, Ready: true, Synchronized: true, Complete: true, Stale: stale})
	}
	if h := observe(start, 20, false, true); h.State != "HEALTHY" {
		t.Fatal(h)
	}
	if h := observe(start.Add(20*time.Second), 100, false, true); h.State == "OVERLOADED" || h.State == "OFFLINE" {
		t.Fatal("brief spike became outage", h)
	}
	for i := 2; i <= 5; i++ {
		observe(start.Add(time.Duration(i)*20*time.Second), 100, false, true)
	}
	if r.Snapshot(start.Add(100 * time.Second))[0].State != "OVERLOADED" {
		t.Fatal("persistent overload not detected")
	}
	if h := observe(start.Add(120*time.Second), 20, true, true); h.State != "TELEMETRY_STALE" {
		t.Fatal("unavailable telemetry became zero demand", h)
	}
	if h := observe(start.Add(140*time.Second), 20, false, true); h.State != "RECOVERING" {
		t.Fatal(h)
	}
	for i := 8; i <= 23; i++ {
		observe(start.Add(time.Duration(i)*20*time.Second), 20, false, true)
	}
	if !r.Safe([]string{"node"}, start.Add(460*time.Second)) {
		t.Fatal("healthy recovery never cleared")
	}
	if r.Safe([]string{"node"}, start.Add(10*time.Minute)) {
		t.Fatal("stale registry authorized new work")
	}
}

func TestRepeatedCPUSampleCannotProveSustainedOverload(t *testing.T) {
	r := NewHealthRegistry()
	at := time.Now().UTC()
	for i := 0; i < 4; i++ {
		h := r.Observe(Observation{NodeID: "node", At: at.Add(time.Duration(i) * 20 * time.Second), ReportedAt: at, CPU: 100, Memory: 20, Connected: true, Ready: true, Synchronized: true, Complete: true})
		if h.State == "OVERLOADED" {
			t.Fatal("one sample was counted repeatedly", h)
		}
	}
}

func TestDisconnectedNodeBecomesOffline(t *testing.T) {
	registry := NewHealthRegistry()
	at := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	observation := Observation{NodeID: "node", At: at, Stale: true}
	if got := registry.Observe(observation); got.State != "TELEMETRY_STALE" {
		t.Fatalf("initial missing telemetry: %s", got.State)
	}
	observation.At = at.Add(time.Minute)
	if got := registry.Observe(observation); got.State != "OFFLINE" {
		t.Fatalf("sustained disconnection: %s", got.State)
	}
}

func TestHealthUsesReceiptTimeDespiteAgentClockSkew(t *testing.T) {
	start := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, skew := range []time.Duration{-4 * time.Minute, 2 * time.Minute} {
		r := NewHealthRegistry()
		for i := 0; i < 4; i++ {
			at := start.Add(time.Duration(i) * 30 * time.Second)
			h := r.Observe(Observation{NodeID: "node", At: at, ReceivedAt: at, ReportedAt: at.Add(skew), CPU: 100, Memory: 20, Connected: true, Ready: true, Synchronized: true, Complete: true})
			if i == 0 && h.State != "HEALTHY" || i == 3 && h.State != "OVERLOADED" {
				t.Fatalf("skew %v observation %d: %+v", skew, i, h)
			}
		}
		r.Remove("node")
		if r.Safe([]string{"node"}, start.Add(90*time.Second)) || len(r.Snapshot(start)) != 0 {
			t.Fatal("removed node remained available")
		}
	}
}
