package iplookup

import (
	"fmt"
	"testing"
)

func ratingReport() Report {
	r := Report{IP: "8.8.8.8", Status: "succeeded", Verdict: "suitable", Facts: Facts{NetworkType: "residential"}, Databases: []Database{{ID: "ipapi", Status: "success"}}, Checkpoint: &Checkpoint{Complete: true, Coverage: 31, Usable: 1}, Proxy: &ProxyAssessment{Status: "success"}, RPKI: &RPKIResult{LiveMeta: LiveMeta{Status: "success"}}, LiveDetails: &LiveDetails{Shodan: ShodanDetails{LiveMeta: LiveMeta{Status: "empty"}}}, Ports: []PortProbe{{Port: 443, Status: "closed"}}, Subnet: &SubnetResult{Status: "success"}}
	for i := range 256 {
		r.Subnet.Addresses = append(r.Subnet.Addresses, SubnetAddress{IP: fmt.Sprintf("8.8.8.%d", i), Status: "clean"})
	}
	return r
}

func TestRatingPrecedenceAndCompleteness(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*Report)
	}{
		{"all passed", "perfect", func(*Report) {}},
		{"neighbor flagged", "excellent", func(r *Report) { r.Subnet.Addresses[0].Status = "flagged" }},
		{"subnet unavailable", "excellent", func(r *Report) { r.Subnet = nil }},
		{"subnet partial", "excellent", func(r *Report) { r.Subnet.Status = "partial" }},
		{"IPv6 no subnet", "perfect", func(r *Report) { r.IP = "2606:4700:4700::1111"; r.Subnet = nil }},
		{"business network", "good", func(r *Report) { r.Facts.NetworkType = "business" }},
		{"business user type", "good", func(r *Report) { v := "business"; r.MaxMind.UserType = &v }},
		{"no probes", "good", func(r *Report) { r.Ports = nil }},
		{"unfinished probes", "good", func(r *Report) { r.Ports[0].Status = "processing" }},
		{"unavailable probes", "perfect", func(r *Report) { r.Ports[0].Status = "error" }},
		{"observed TCP", "good", func(r *Report) {
			r.Ports[0].Status = "success"
			r.Ports[0].Tasks = []ProbeTask{{Key: "tcp", Status: "success"}}
		}},
		{"closed TCP", "perfect", func(r *Report) { r.Ports[0].Tasks = []ProbeTask{{Key: "tcp", Status: "success"}} }},
		{"observed TLS", "good", func(r *Report) {
			r.Ports[0].Status = "success"
			r.Ports[0].Findings = []ProbeFinding{{Protocol: "tls", Confidence: "observed"}}
		}},
		{"suspected proxy", "good", func(r *Report) {
			r.Ports[0].Status = "success"
			r.Ports[0].Findings = []ProbeFinding{{Protocol: "trojan", Confidence: "suspected"}}
		}},
		{"datacenter", "average", func(r *Report) { r.Warnings = []Refusal{{Kind: "datacenter", Source: "ipapi", Value: true}} }},
		{"datacenter business", "average", func(r *Report) { r.Facts.NetworkType = "datacenter"; v := "business"; r.MaxMind.UserType = &v }},
		{"provider incomplete", "incomplete", func(r *Report) { r.Checkpoint.Complete = false }},
		{"unknown coverage", "incomplete", func(r *Report) { r.Checkpoint.Coverage = 15 }},
		{"proxy incomplete", "incomplete", func(r *Report) { r.Proxy.Status = "partial" }},
		{"RPKI unavailable", "incomplete", func(r *Report) { r.RPKI.Status = "error" }},
		{"Shodan unavailable", "incomplete", func(r *Report) { r.LiveDetails.Shodan.Status = "error" }},
		{"datacenter incomplete", "incomplete", func(r *Report) { r.Warnings = []Refusal{{Kind: "datacenter"}}; r.Proxy = nil }},
		{"proxy beats datacenter", "poor", func(r *Report) { r.Facts.NetworkType = "datacenter"; r.Proxy.Matches = []string{"list"} }},
		{"confirmed probe beats missing core", "poor", func(r *Report) {
			r.Proxy = nil
			r.Ports[0].Findings = []ProbeFinding{{Protocol: "socks5", Confidence: "confirmed"}}
		}},
		{"tag beats missing core", "poor", func(r *Report) { r.Checkpoint.Complete = false; r.LiveDetails.Shodan.Tags = []string{"vpn"} }},
		{"vulnerability", "poor", func(r *Report) { r.LiveDetails.Shodan.Vulns = []string{"CVE-2026-1234"} }},
		{"abuse", "poor", func(r *Report) { r.Refusals = []Refusal{{Kind: "abuse", Source: "ipapi", Value: true}} }},
		{"invalid RPKI", "poor", func(r *Report) { r.Refusals = []Refusal{{Kind: "rpki_invalid_asn", Source: "ripe_rpki", Value: "AS1"}} }},
		{"ancillary outage", "perfect", func(r *Report) {
			r.Status = "partial"
			r.LiveDetails.Transfers = &TransferHistory{LiveMeta: LiveMeta{Status: "error"}}
		}},
	}
	cases = append(cases, []struct {
		name, want string
		change     func(*Report)
	}{
		{"legacy invalid origin without refusal", "poor", func(r *Report) {
			r.Checkpoint = nil
			r.RPKI.Routes = []RPKIRoute{{LiveMeta: LiveMeta{Status: "success"}, ASN: 1, Validity: "invalid_asn"}}
		}},
		{"legacy invalid length without refusal", "poor", func(r *Report) {
			r.Checkpoint = nil
			r.RPKI.Routes = []RPKIRoute{{LiveMeta: LiveMeta{Status: "success"}, ASN: 1, Validity: "invalid_length"}}
		}},
		{"legacy provider outage", "incomplete", func(r *Report) {
			r.Checkpoint = nil
			r.Databases[0].Status = "error"
		}},
	}...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ratingReport()
			tc.change(&r)
			updateRating(&r)
			if r.Rating != tc.want {
				t.Fatalf("rating=%s want=%s", r.Rating, tc.want)
			}
		})
	}
}

func TestLegacyRatingProjectionDoesNotPromoteMissingEvidence(t *testing.T) {
	r := Report{Status: "succeeded", Verdict: "unsuitable", Refusals: []Refusal{{Kind: "datacenter", Source: "ipapi", Value: "Hosting"}}}
	public := PublicReport(r)
	if public.Rating != "incomplete" || len(public.Warnings) != 1 || len(public.Refusals) != 0 || public.Verdict != "unsuitable" {
		t.Fatalf("legacy projection: %+v", public)
	}
	if len(r.Refusals) != 1 || len(r.Warnings) != 0 {
		t.Fatal("projection mutated the frozen report")
	}
	r = ratingReport()
	r.Rating = "perfect"
	r.Checkpoint = nil
	raw, err := EncodeReport(r)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeReport(raw)
	if err != nil || PublicReport(restored).Rating != "perfect" {
		t.Fatal("rating did not survive the compact codec")
	}
}
