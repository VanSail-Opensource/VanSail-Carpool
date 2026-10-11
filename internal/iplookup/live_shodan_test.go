package iplookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestLiveInternetDBObservationsAndNoInformation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		state  string
		ports  int
	}{
		{"observed", 200, `{"ip":"1.1.1.1","hostnames":["one.one.one.one","one.one.one.one"],"ports":[443,53,443],"cpes":[],"tags":[],"vulns":[]}`, "success", 2},
		{"not scanned", 404, `{"detail":"No information available"}`, "empty", 0},
		{"provider failure", 500, `{}`, "error", 0},
		{"wrong address", 200, `{"ip":"8.8.8.8","hostnames":[],"ports":[443],"tags":["vpn"],"vulns":["CVE-2017-15906"],"cpes":[]}`, "error", 0},
		{"missing ports", 200, `{"ip":"1.1.1.1","hostnames":[]}`, "error", 0},
		{"invalid port", 200, `{"ip":"1.1.1.1","hostnames":[],"ports":[0,65536,443]}`, "partial", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := liveTestService(t, "shodan", func(r *http.Request) (int, string) {
				if r.URL.Host != "internetdb.shodan.io" || r.URL.Path != "/1.1.1.1" || r.URL.RawQuery != "" {
					t.Error("InternetDB request mismatch")
				}
				return tc.status, tc.body
			})
			r := s.shodan(context.Background(), "1.1.1.1")
			if r.Status != tc.state || len(r.Ports) != tc.ports || r.ObservedAt != "" {
				t.Fatalf("%+v", r)
			}
			if tc.name == "observed" && (r.Ports[0] != 53 || len(r.Hostnames) != 1) {
				t.Fatal("sorting/deduplication failed")
			}
			if tc.name == "wrong address" && (len(r.Tags) != 0 || len(r.Vulns) != 0) {
				t.Fatal("published findings for another address")
			}
		})
	}
}

func TestLiveInternetDBQualityFields(t *testing.T) {
	for _, tc := range []struct {
		name, fields, state string
		cpes, tags, vulns   int
	}{
		{"empty", `"cpes":[],"tags":[],"vulns":[]`, "empty", 0, 0, 0},
		{"tag only", `"cpes":[],"tags":["vpn"],"vulns":[]`, "success", 0, 1, 0},
		{"vulnerability only", `"cpes":[],"tags":[],"vulns":["CVE-2017-15906"]`, "success", 0, 0, 1},
		{"software only", `"cpes":["cpe:/a:openbsd:openssh:7.4"],"tags":[],"vulns":[]`, "success", 1, 0, 0},
		{"normalized", `"cpes":[" cpe:/a:igor_sysoev:nginx ","cpe:/a:igor_sysoev:nginx"],"tags":["vpn","vpn"],"vulns":["CVE-2017-15906","CVE-2017-15906"]`, "success", 1, 1, 1},
		{"malformed software retains findings", `"cpes":null,"tags":["vpn"],"vulns":["CVE-2017-15906"]`, "partial", 0, 1, 1},
		{"mixed items retain valid findings", `"cpes":[],"tags":["vpn",42," "],"vulns":[]`, "partial", 0, 1, 0},
		{"missing field", `"cpes":[],"tags":["vpn"]`, "partial", 0, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := liveTestService(t, "shodan", func(*http.Request) (int, string) {
				return 200, `{"ip":"1.1.1.1","ports":[],"hostnames":[],` + tc.fields + `}`
			})
			r := s.shodan(context.Background(), "1.1.1.1")
			if r.Status != tc.state || len(r.CPEs) != tc.cpes || len(r.Tags) != tc.tags || len(r.Vulns) != tc.vulns {
				t.Fatalf("unexpected quality fields: %+v", r)
			}
		})
	}
}

func TestLiveInternetDBPreservesAllCPEsAndFindingsOnPortError(t *testing.T) {
	cpes := make([]string, 300)
	for i := range cpes {
		cpes[i] = fmt.Sprintf("cpe:/a:vendor:product_%03d:1.0", i)
	}
	encoded, err := json.Marshal(cpes)
	if err != nil {
		t.Fatal(err)
	}
	s := liveTestService(t, "shodan", func(*http.Request) (int, string) {
		return 200, `{"ip":"1.1.1.1","hostnames":[],"cpes":` + string(encoded) + `,"tags":["vpn"],"vulns":[]}`
	})
	r := s.shodan(context.Background(), "1.1.1.1")
	if len(r.CPEs) != 300 || r.CPEs[299] != cpes[299] || len(r.Tags) != 1 || r.Status != "error" {
		t.Fatalf("lost findings or CPEs: %+v", r)
	}
}
