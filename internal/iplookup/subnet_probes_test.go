package iplookup

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

func subnetProbeService(t *testing.T) *EnhancedService {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "subnet-probes", Capacity: 4, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Shutdown(context.Background()) })
	return NewEnhancedService(nil, map[string]*upstreamqueue.Queue{"probes": queue}, nil)
}

func TestSubnetProbesRunAfterPrimaryFailureWithTCPAndRedPrecedence(t *testing.T) {
	s := subnetProbeService(t)
	r := Report{IP: "8.8.8.8", Status: "processing", Refusals: []Refusal{{Kind: "abuse_reports"}}, Subnet: &SubnetResult{Reason: "ip_not_passed", Addresses: []SubnetAddress{
		{IP: "8.8.8.8", Status: "clean"}, {IP: "8.8.8.9", Status: "flagged"},
		{IP: "8.8.8.10", Status: "clean"}, {IP: "8.8.8.11", Status: "unknown"},
		{IP: "8.8.8.12", Status: "unknown"}, {IP: "8.8.8.13", Status: "clean"},
	}}}
	var mu sync.Mutex
	calls := map[string][]string{}
	probe := func(_ context.Context, ip string, port int, key string, _ Config, _ func(int) error) probeOutcome {
		mu.Lock()
		defer mu.Unlock()
		if ip == "8.8.8.8" || ip == "8.8.8.9" || (port != 80 && port != 443 && port != 8443) {
			t.Errorf("unexpected address/port: %s:%d", ip, port)
		}
		id := fmt.Sprintf("%s:%d", ip, port)
		if key != "tcp" && (len(calls[id]) == 0 || calls[id][0] != "tcp") {
			t.Errorf("probe preceded TCP: %s %s", id, key)
		}
		calls[id] = append(calls[id], key)
		if key == "tcp" {
			if ip == "8.8.8.13" && port == 80 {
				return probeOutcome{status: "error"}
			}
			if port != 80 || ip == "8.8.8.12" {
				return probeOutcome{status: "closed"}
			}
		}
		if key == "reality" && (ip == "8.8.8.10" || ip == "8.8.8.11") {
			return probeOutcome{status: "success", findings: []ProbeFinding{{Protocol: "reality", Confidence: "suspected"}}}
		}
		if key == "cloudflare" && ip == "8.8.8.11" {
			if r.Subnet.Addresses[3].Status != "suspected" {
				t.Error("Cloudflare did not follow the suspected Reality transition")
			}
			return probeOutcome{status: "success", findings: []ProbeFinding{{Protocol: "cloudflare", Confidence: "confirmed"}}}
		}
		return probeOutcome{status: "success"}
	}
	save := func() error { _, err := EncodeReport(r); return err }
	if err := s.runSubnetProbes(context.Background(), &r, Config{}, save, probe); err != nil {
		t.Fatal(err)
	}
	for i, expected := range []string{"flagged", "flagged", "suspected", "flagged", "unknown", "clean"} {
		if r.Subnet.Addresses[i].Status != expected {
			t.Fatalf("row %d status=%s expected=%s", i, r.Subnet.Addresses[i].Status, expected)
		}
	}
	if r.Subnet.Addresses[3].Ports[1].Status != "skipped" || r.Subnet.Addresses[4].Ports[0].Tasks[1].Status != "skipped" {
		t.Fatal("confirmed red/closed ports did not skip later work")
	}
	before := len(calls)
	if err := s.runSubnetProbes(context.Background(), &r, Config{}, save, func(context.Context, string, int, string, Config, func(int) error) probeOutcome {
		t.Error("completed probe repeated")
		return probeOutcome{status: "error"}
	}); err != nil || len(calls) != before {
		t.Fatalf("resume failed: %v", err)
	}
}

func TestSubnetProbeCheckpointResumesOnlyUnfinishedTasks(t *testing.T) {
	s := subnetProbeService(t)
	r := Report{IP: "8.8.8.8", Status: "processing", Subnet: &SubnetResult{Addresses: []SubnetAddress{{IP: "8.8.8.8", Status: "clean"}}}}
	interrupted := errors.New("checkpoint interrupted")
	save := func() error {
		for _, port := range r.Subnet.Addresses[0].Ports {
			for _, task := range port.Tasks {
				if task.Key == "reality" && task.Status == "active" {
					return interrupted
				}
			}
		}
		return nil
	}
	probe := func(context.Context, string, int, string, Config, func(int) error) probeOutcome {
		return probeOutcome{status: "success"}
	}
	if err := s.runSubnetProbes(context.Background(), &r, Config{}, save, probe); !errors.Is(err, interrupted) {
		t.Fatalf("missing interruption: %v", err)
	}
	raw, err := EncodeReport(r)
	if err != nil {
		t.Fatal(err)
	}
	r, err = DecodeReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	probe = func(_ context.Context, _ string, port int, key string, _ Config, _ func(int) error) probeOutcome {
		if port == 80 && key == "tcp" {
			t.Error("completed TCP repeated after decoding")
		}
		return probeOutcome{status: "success"}
	}
	if err := s.runSubnetProbes(context.Background(), &r, Config{}, func() error { return nil }, probe); err != nil {
		t.Fatal(err)
	}
}

func TestSubnetRealityDoesNotDowngradeConfirmedAbuse(t *testing.T) {
	row := SubnetAddress{Status: "flagged", Ports: []PortProbe{{Findings: []ProbeFinding{{Protocol: "reality", Confidence: "suspected"}}}}}
	applySubnetProbeFinding(&row)
	if row.Status != "flagged" {
		t.Fatal("Reality downgraded confirmed abuse")
	}
}

func TestSubnetCloudflareAtRequestedAddressAddsPortRefusal(t *testing.T) {
	s := subnetProbeService(t)
	r := Report{IP: "8.8.8.8", Status: "processing", Subnet: &SubnetResult{Addresses: []SubnetAddress{{IP: "8.8.8.8", Status: "clean"}}}}
	probe := func(_ context.Context, _ string, _ int, key string, _ Config, _ func(int) error) probeOutcome {
		out := probeOutcome{status: "success"}
		if key == "reality" {
			out.findings = []ProbeFinding{{Protocol: "reality", Confidence: "suspected"}}
		}
		if key == "cloudflare" {
			out.findings = []ProbeFinding{{Protocol: "cloudflare", Confidence: "confirmed"}}
		}
		return out
	}
	if err := s.runSubnetProbes(context.Background(), &r, Config{}, func() error { return nil }, probe); err != nil {
		t.Fatal(err)
	}
	if r.Subnet.Addresses[0].Status != "flagged" || len(r.Refusals) != 1 || r.Refusals[0].Kind != "cloudflare_cdn" || r.Refusals[0].Value != 80 {
		t.Fatalf("requested-IP confirmation not retained: %+v", r.Refusals)
	}
}
