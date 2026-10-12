package iplookup

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProviderVariantsAndRiskScope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, id, raw              string
		wantRisk, complete, failed bool
	}{
		{"Scamalytics low score passes", "scamalytics", `{"scamalytics":{"status":"ok","ip":"150.249.241.62","scamalytics_score":5,"scamalytics_risk":"low","scamalytics_isp":"PREMIUM FIELD - upgrade to view","scamalytics_proxy":{"is_datacenter":false,"is_vpn":false},"is_blacklisted_external":false},"external_datasources":{"firehol":{"is_proxy":false},"x4bnet":{"is_tor":false}}}`, false, true, false},
		{"Scamalytics premium placeholder", "scamalytics", `{"scamalytics":{"status":"ok","ip":"150.249.241.62","scamalytics_score":0,"scamalytics_risk":"low","scamalytics_isp_score":5,"scamalytics_isp":"PREMIUM FIELD - upgrade to view","scamalytics_proxy":{"is_datacenter":false,"is_vpn":false},"is_blacklisted_external":false},"external_datasources":{"firehol":{"is_proxy":false},"x4bnet":{"is_tor":false},"ipinfo":{"as_name":"Sony Network Communications Inc."}}}`, false, true, false},
		{"Scamalytics outage", "scamalytics", `{"scamalytics":{"status":"error","error":"credits exhausted"}}`, false, false, true},
		{"Scamalytics medium fails", "scamalytics", `{"scamalytics":{"status":"ok","ip":"150.249.241.62","scamalytics_score":25,"scamalytics_risk":"medium","scamalytics_proxy":{"is_datacenter":false,"is_vpn":false},"is_blacklisted_external":false},"external_datasources":{"firehol":{"is_proxy":false},"x4bnet":{"is_tor":false}}}`, true, true, false},
		{"Scamalytics residential proxy", "scamalytics", `{"scamalytics":{"status":"ok","ip":"150.249.241.62","scamalytics_score":0,"scamalytics_risk":"low","scamalytics_proxy":{"is_datacenter":false,"is_vpn":false,"is_resproxy":true},"is_blacklisted_external":false},"external_datasources":{"firehol":{"is_proxy":false},"x4bnet":{"is_tor":false}}}`, true, true, false},
		{"clean ISP", "abuseipdb", `{"data":{"ipAddress":"150.249.241.62","usageType":"Fixed Line ISP","abuseConfidenceScore":0,"totalReports":0,"isTor":false}}`, false, true, false},
		{"one report", "abuseipdb", `{"data":{"ipAddress":"150.249.241.62","usageType":"Fixed Line ISP","abuseConfidenceScore":0,"totalReports":1,"isTor":false}}`, true, true, false},
		{"network ratios contextual", "ipapi", `{"ip":"150.249.241.62","is_abuser":false,"is_datacenter":false,"is_vpn":false,"is_proxy":false,"is_tor":false,"company":{"abuser_score":"0.0002 (Very Low)"},"asn":{"abuser_score":"0.0003 (Very Low)"}}`, false, true, false},
		{"truthy VPN", "ipapi", `{"ip":"150.249.241.62","is_abuser":false,"is_datacenter":false,"is_vpn":"detected","is_proxy":false,"is_tor":false}`, true, true, false},
		{"missing keyed signals", "ipapi", `{"ip":"150.249.241.62","company":{"name":"ISP"}}`, false, false, false},
		{"new anonymizer hosting warning", "maxmind", `{"traits":{"ip_address":"150.249.241.62","user_type":"residential"},"anonymizer":{"is_hosting_provider":true}}`, false, true, false},
		{"legacy anonymizer", "maxmind", `{"traits":{"ip_address":"150.249.241.62","is_anonymous_vpn":true}}`, true, true, false},
		{"true-only omission", "maxmind", `{"traits":{"ip_address":"150.249.241.62","user_type":"residential"}}`, false, true, false},
		{"malformed anonymizer", "maxmind", `{"traits":{"ip_address":"150.249.241.62","user_type":"residential"},"anonymizer":{"is_hosting_provider":"unknown"}}`, false, false, false},
		{"residential proxy object", "maxmind", `{"traits":{"ip_address":"150.249.241.62"},"anonymizer":{"residential":{"confidence":40}}}`, true, true, false},
		{"IPQS free connection type", "ipqs", `{"success":true,"fraud_score":0,"recent_abuse":false,"proxy":false,"vpn":false,"tor":false,"connection_type":"Premium required."}`, false, true, false},
		{"positive fraud", "ipqs", `{"success":true,"fraud_score":1,"recent_abuse":false,"proxy":false,"vpn":false,"tor":false}`, true, true, false},
		{"null fraud", "ipqs", `{"success":true,"fraud_score":null,"recent_abuse":false,"proxy":false,"vpn":false,"tor":false}`, false, false, false},
		{"provider rejection", "ipqs", `{"success":false,"message":"quota"}`, false, false, true},
		{"IP2 free tier", "ip2location", `{"ip":"150.249.241.62","is_proxy":false,"country_code":"JP"}`, false, false, false},
		{"IP2 datacenter warning", "ip2location", `{"ip":"150.249.241.62","usage_type":"DCH"}`, false, false, false},
		{"wrong IP", "ipapi", `{"ip":"8.8.8.8"}`, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body object
			if err := json.Unmarshal([]byte(tc.raw), &body); err != nil {
				t.Fatal(err)
			}
			p, err := parseProvider(tc.id, "150.249.241.62", body)
			if (err != nil) != tc.failed {
				t.Fatalf("error=%v, want failed=%v", err, tc.failed)
			}
			if !tc.failed && ((len(RiskReasons(p)) > 0) != tc.wantRisk || p.Complete != tc.complete) {
				t.Fatalf("result=%+v, want risk=%v complete=%v", p, tc.wantRisk, tc.complete)
			}
			if !tc.failed && p.Signals.Datacenter != nil && *p.Signals.Datacenter && len(providerWarnings(p)) != 1 {
				t.Fatal("confirmed hosting evidence lost its warning")
			}
		})
	}
}

func TestCanonicalPublicIP(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"127.0.0.1", "10.1.2.3", "100.64.0.1", "192.0.2.1", "198.18.0.1", "224.0.0.1", "2001:db8::1", "fe80::1%eth0", "::1", "https://8.8.8.8", "8.8.8.0/24", "example.com"} {
		if _, err := CanonicalIP(raw); err == nil {
			t.Errorf("accepted non-public input %q", raw)
		}
	}
	for _, raw := range []string{"150.249.241.62", "2606:4700:4700::1111", "::ffff:150.249.241.62"} {
		if _, err := CanonicalIP(raw); err != nil {
			t.Errorf("rejected public input %q: %v", raw, err)
		}
	}
}

func TestVerdictRequiresCoverageAndResidentialEvidence(t *testing.T) {
	t.Parallel()
	clean := ProviderResult{ID: "ipapi", Status: "success", Complete: true, Facts: Facts{NetworkType: "mobile"}, Signals: Signals{Abuse: boolean(false), Datacenter: boolean(false), VPN: boolean(false), Proxy: boolean(false), Tor: boolean(false)}}
	for _, tc := range []struct {
		name            string
		results         []ProviderResult
		verdict, status string
	}{
		{"mobile", []ProviderResult{clean}, "suitable", "succeeded"},
		{"outage", []ProviderResult{clean, {ID: "maxmind", Status: "error"}}, "inconclusive", "partial"},
		{"all unavailable", []ProviderResult{{ID: "ipapi", Status: "error"}}, "inconclusive", "partial"},
		{"risk wins", []ProviderResult{clean, {ID: "maxmind", Status: "partial", Signals: Signals{Proxy: boolean(true)}}}, "unsuitable", "partial"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Aggregate(Report{Providers: tc.results}, time.Now())
			if r.Verdict != tc.verdict || r.Status != tc.status {
				t.Fatalf("report=%+v", r)
			}
		})
	}
}
