package iplookup

import (
	"reflect"
	"testing"
	"time"
)

func TestShodanFindingsRefuseQualityAndSurvivePersistence(t *testing.T) {
	for _, tc := range []struct {
		name, status, verdict string
		tags, vulns           []string
		refusals              int
	}{
		{"software only", "success", "suitable", nil, nil, 0},
		{"any tag", "success", "unsuitable", []string{"self-signed"}, nil, 1},
		{"any vulnerability", "success", "unsuitable", nil, []string{"CVE-2017-15906"}, 1},
		{"all findings", "success", "unsuitable", []string{"vpn", "vpn", "proxy"}, []string{"CVE-2017-15906", "CVE-2018-15473"}, 4},
		{"partial findings", "partial", "unsuitable", []string{"vpn"}, nil, 1},
		{"error with evidence", "error", "unsuitable", nil, []string{"CVE-2017-15906"}, 1},
		{"unavailable without findings", "error", "suitable", nil, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Report{ID: "shodan-quality", IP: "1.1.1.1", Status: "succeeded", Verdict: "suitable", Refusals: []Refusal{}, Proxy: &ProxyAssessment{Status: "success"}, Ports: []PortProbe{{Port: 443, Status: "closed"}}, LiveDetails: &LiveDetails{IP: "1.1.1.1", Shodan: ShodanDetails{LiveMeta: LiveMeta{Status: tc.status, Source: "shodan:internetdb"}, CPEs: []string{"cpe:/a:openbsd:openssh:7.4"}, Tags: tc.tags, Vulns: tc.vulns}}}
			applyEnhancedVerdict(&r)
			applyEnhancedVerdict(&r)
			if r.Verdict != tc.verdict || len(r.Refusals) != tc.refusals {
				t.Fatalf("wrong quality or duplicate refusals: %+v", r)
			}
			if (tc.status == "partial" || tc.status == "error") && r.Status != "partial" {
				t.Fatal("missing incomplete coverage")
			}
			encoded, err := EncodeReport(r)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeReport(encoded)
			if err != nil {
				t.Fatal(err)
			}
			public := PublicReport(decoded)
			if public.Verdict != tc.verdict || !reflect.DeepEqual(public.Refusals, r.Refusals) || !reflect.DeepEqual(public.LiveDetails.Shodan.CPEs, r.LiveDetails.Shodan.CPEs) || len(public.LiveDetails.Shodan.Tags) != len(tc.tags) || len(public.LiveDetails.Shodan.Vulns) != len(tc.vulns) {
				t.Fatalf("lost persisted evidence: %+v", public)
			}
		})
	}
}

func TestShodanRefusalSurvivesAggregationAndUnrelatedFailures(t *testing.T) {
	r := NewReport("shodan-partial", "1.1.1.1", DefaultConfig())
	r.LiveDetails = &LiveDetails{Shodan: ShodanDetails{LiveMeta: LiveMeta{Status: "partial"}, Tags: []string{"vpn"}}}
	r = Aggregate(r, time.Now())
	if r.Verdict != "unsuitable" || r.Status != "partial" || len(r.Refusals) != 1 || r.RefundRequired {
		t.Fatalf("Shodan refusal lost to incomplete coverage: %+v", r)
	}
}
