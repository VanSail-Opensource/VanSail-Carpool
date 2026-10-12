package iplookup

import (
	"strings"
	"testing"
	"time"
)

func TestScamalyticsRetainsSelectedEnrichmentAndRefusalData(t *testing.T) {
	t.Parallel()
	p := parseFixture(t, "scamalytics", `{"scamalytics":{"status":"ok","ip":"8.8.8.8","scamalytics_score":0,"scamalytics_risk":"low","scamalytics_proxy":{"is_datacenter":false,"is_vpn":false},"is_blacklisted_external":false},"external_datasources":{"firehol":{"is_proxy":false},"x4bnet":{"is_tor":false},"maxmind_geolite2":{"ip_country_code":"US","ip_city":"City A","ip_geolocation":"0,0","ip_location_accuracy_km":"20","asn":"15169","as_name":"Google LLC"},"dbip":{"ip_country_code":"GB","ip_city":"City B","ip_geolocation":"50,1"},"residential_proxy_DBI":{"is_resproxy":true,"last_proxy_provider":"Proxywing residential"}}}`)
	if p.Geo.Source != "scamalytics:maxmind_geolite2" || p.Geo.City != "City A" || p.Geo.Latitude == nil || *p.Geo.Latitude != 0 || p.Facts.ASNName != "Google LLC" {
		t.Fatalf("provider=%+v geo=%+v", p, p.Geo)
	}
	r := NewReport("r", "8.8.8.8", DefaultConfig())
	RecordProvider(&r, p, DefaultConfig())
	r = Aggregate(r, time.Now())
	if r.Facts.Country != "US" || r.Facts.City != "City A" || r.Sources["latitude"] != "scamalytics:maxmind_geolite2" || r.Sources["asnName"] != "scamalytics:maxmind_geolite2" {
		t.Fatalf("mixed projection: %+v", r)
	}
	found := false
	for _, refusal := range r.Refusals {
		if refusal.Kind == "proxy" && refusal.Source == "scamalytics:residential_proxy_DBI" && refusal.Value == "Proxywing residential" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing proxy evidence: %v", r.Refusals)
	}
	encoded, err := EncodeReport(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "City B") || strings.Contains(string(encoded), "ip_location_accuracy_km") || strings.Contains(string(encoded), "scamalytics_proxy") {
		t.Fatalf("retained unselected provider data: %s", encoded)
	}
}

func TestDocumentedNetworkCodesDoNotTreatSearchBotsAsSatellite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ code, want string }{{"SES", ""}, {"AIC", ""}, {"RSV", ""}, {"SAT", "satellite"}, {"CDN", "datacenter"}, {"ORG", "business"}, {"MIL", "business"}, {"LIB", "business"}, {"ISP/MOB", "mobile"}} {
		if got := networkType(tc.code); got != tc.want {
			t.Fatalf("code=%s type=%s want=%s", tc.code, got, tc.want)
		}
	}
}

func TestRefusalAfterOutageStaysPartialWithoutRefund(t *testing.T) {
	t.Parallel()
	r := NewReport("r", "8.8.8.8", DefaultConfig())
	RecordProvider(&r, ProviderResult{ID: "abuseipdb", Status: "error"}, DefaultConfig())
	p := parseFixture(t, "ipapi", `{"ip":"8.8.8.8","is_datacenter":true,"is_vpn":true,"datacenter":{"datacenter":"HostPapa"},"vpn":{"service":"NordVPN"}}`)
	RecordProvider(&r, p, DefaultConfig())
	r = Aggregate(r, time.Now())
	if r.Status != "partial" || r.Verdict != "unsuitable" || r.RefundRequired {
		t.Fatalf("report=%+v", r)
	}
	if len(r.Refusals) != 1 || r.Refusals[0].Value != "NordVPN" || len(r.Warnings) != 1 || r.Warnings[0].Value != "HostPapa" {
		t.Fatalf("lost refusal data: %v", r.Refusals)
	}
}
