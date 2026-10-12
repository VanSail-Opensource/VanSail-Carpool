package iplookup

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProbeGateAndSuspectedFindings(t *testing.T) {
	cases := []struct {
		name, status, confidence string
		pass                     bool
	}{{"closed", "closed", "", true}, {"clean", "success", "", true}, {"heuristic", "success", "suspected", true}, {"confirmed", "success", "confirmed", false}, {"unavailable", "error", "", true}, {"active", "processing", "", false}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := PortProbe{Port: 443, Status: tc.status}
			if tc.confidence != "" {
				p.Findings = []ProbeFinding{{Protocol: "trojan", Confidence: tc.confidence}}
			}
			if got := probesPassed([]PortProbe{p}); got != tc.pass {
				t.Fatalf("gate %v", got)
			}
		})
	}
}
func TestNeighborAbuseDoesNotRefuseRequestedIP(t *testing.T) {
	r := Report{Verdict: "suitable", Status: "succeeded", Proxy: &ProxyAssessment{Status: "success"}, Ports: []PortProbe{{Port: 443, Status: "closed"}}, Subnet: &SubnetResult{Status: "success", Addresses: []SubnetAddress{{IP: "8.8.8.7", Status: "flagged"}}}}
	applyEnhancedVerdict(&r)
	if r.Verdict != "suitable" {
		t.Fatal("neighbor changed suitability")
	}
	r.Proxy.Matches = []string{"named_list"}
	applyEnhancedVerdict(&r)
	if r.Verdict != "unsuitable" {
		t.Fatal("proxy match did not refuse")
	}
}

func TestConfirmedServicesCombinePerPortAndCloudflareRefuses(t *testing.T) {
	r := Report{Verdict: "suitable", Proxy: &ProxyAssessment{Status: "success"}, Ports: []PortProbe{
		{Port: 443, Status: "success", Findings: []ProbeFinding{
			{Protocol: "socks5", Confidence: "confirmed"},
			{Protocol: "socks5", Confidence: "confirmed"},
			{Protocol: "http", Confidence: "confirmed"},
			{Protocol: "cloudflare", Confidence: "confirmed"},
		}},
		{Port: 8443, Status: "success", Findings: []ProbeFinding{{Protocol: "socks5", Confidence: "confirmed"}}},
	}, Refusals: []Refusal{{Kind: "active_proxy", Source: "http", Value: float64(443)}, {Kind: "active_proxy", Source: "socks5", Value: json.Number("443")}}}
	applyEnhancedVerdict(&r)
	applyEnhancedVerdict(&r)
	want := []Refusal{
		{Kind: "active_proxy", Source: "socks5,http", Value: 443},
		{Kind: "cloudflare_cdn", Source: "cloudflare", Value: 443},
		{Kind: "active_proxy", Source: "socks5", Value: 8443},
	}
	if r.Verdict != "unsuitable" || r.Rating != "poor" || !reflect.DeepEqual(r.Refusals, want) || probesPassed(r.Ports) {
		t.Fatalf("wrong confirmed evidence: %+v", r)
	}
}
