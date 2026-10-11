package iplookup

import "testing"

func TestProbeGateAndSuspectedFindings(t *testing.T) {
	cases := []struct {
		name, status, confidence string
		pass                     bool
	}{{"closed", "closed", "", true}, {"clean", "success", "", true}, {"heuristic", "success", "suspected", true}, {"confirmed", "success", "confirmed", false}, {"unavailable", "error", "", false}, {"active", "processing", "", false}}
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
