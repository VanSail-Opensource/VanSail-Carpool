package iplookup

import "testing"

func TestRealityDNSMatchAndIndependentMismatchEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		obs                  []DNSObservation
		configured           int
		protocol, confidence string
	}{
		{"match despite outage", []DNSObservation{{Server: "one", Status: "success", Addresses: []string{"8.8.8.8"}}, {Server: "two", Status: "error"}}, 2, "web_server", "observed"},
		{"one mismatch insufficient", []DNSObservation{{Server: "one", Status: "success", Addresses: []string{"1.1.1.1"}}}, 3, "reality", "unavailable"},
		{"duplicate resolver insufficient", []DNSObservation{{Server: "one", Status: "success", Addresses: []string{"1.1.1.1"}}, {Server: "one", Status: "success", Addresses: []string{"1.1.1.1"}}}, 3, "reality", "unavailable"},
		{"two mismatch", []DNSObservation{{Server: "one", Status: "success", Addresses: []string{"1.1.1.1"}}, {Server: "two", Status: "success", Addresses: []string{"9.9.9.9"}}}, 3, "reality", "suspected"},
		{"sole configured", []DNSObservation{{Server: "one", Status: "success", Addresses: []string{"1.1.1.1"}}}, 1, "reality", "suspected"},
		{"no address", []DNSObservation{{Server: "one", Status: "no_address"}}, 3, "reality", "observed"},
		{"no healthy resolvers", nil, 3, "reality", "unavailable"},
		{"empty answers", []DNSObservation{{Server: "one", Status: "success"}, {Server: "two", Status: "success"}}, 3, "reality", "observed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := classifyReality("8.8.8.8", "example.net", tc.obs, tc.configured)
			if r.Protocol != tc.protocol || r.Confidence != tc.confidence {
				t.Fatalf("classification %+v", r)
			}
		})
	}
}

func TestTrojanDistinctProfilesAreRequiredOnSamePort(t *testing.T) {
	p := PortProbe{Port: 443, Status: "success", Tasks: []ProbeTask{{Key: "trojan_alpn", Result: "detected"}}}
	o := probeOutcome{status: "success", findings: []ProbeFinding{{Protocol: "trojan", Evidence: "alpn_http_mismatch"}}}
	recordProbeOutcome(&p, "trojan_alpn", o)
	recordProbeOutcome(&p, "trojan_alpn", o)
	if len(p.Findings) != 1 || p.Findings[0].Confidence != "observed" {
		t.Fatal("replay counted as another profile")
	}
	other := PortProbe{Port: 8443, Tasks: []ProbeTask{{Key: "trojan_close", Result: "detected"}}}
	recordProbeOutcome(&other, "trojan_close", o)
	if other.Findings[0].Confidence != "observed" {
		t.Fatal("different port contributed suspicion")
	}
	p.Tasks = append(p.Tasks, ProbeTask{Key: "trojan_close", Result: "detected"})
	recordProbeOutcome(&p, "trojan_close", o)
	if p.Findings[0].Confidence != "suspected" || p.Findings[1].Confidence != "suspected" {
		t.Fatal("two independent profiles did not suspect")
	}
	if !probesPassed([]PortProbe{{Status: "success", Tasks: []ProbeTask{{Key: "trojan_short", Status: "success", Result: "unavailable"}}}}) {
		t.Fatal("finished unavailable Trojan blocked completion")
	}
}
