package iplookup

import (
	"net/netip"
	"slices"
	"testing"
)

func TestFirewallOverlayUpdatesCachedReportWithoutRewritingFacts(t *testing.T) {
	service := NewEnhancedService(nil, nil, nil)
	base := Report{IP: "8.8.8.8", Status: "succeeded", Verdict: "suitable", RefundRequired: true, Proxy: &ProxyAssessment{Status: "success", Sources: []ProxySourceResult{}, Matches: []string{}}, Subnet: &SubnetResult{Sources: []InventorySource{}, Addresses: []SubnetAddress{{IP: "8.8.8.8", Status: "clean", Proxy: "success", Abuse: "success", IPAPI: "success", Sources: []string{}}}}}
	service.SetDetectedPrefixes([]string{"8.8.8.0/24"})
	blocked := service.EffectiveReport(base, "8.8.8.8")
	if blocked.Verdict != "unsuitable" || !slices.Contains(blocked.Proxy.Matches, detectedSourceID) || blocked.Proxy.Sources[0].Category != AbuseCategory || blocked.Subnet.Addresses[0].Abuse != "flagged" {
		t.Fatal("live ban not classified as Abuse")
	}
	if len(base.Proxy.Matches) != 0 || len(base.Subnet.Addresses[0].Sources) != 0 || base.Verdict != "suitable" {
		t.Fatal("overlay mutated cached provider facts")
	}
	service.SetDetectedPrefixes(nil)
	released := service.EffectiveReport(base, "8.8.8.8")
	if released.Verdict != "suitable" || slices.Contains(released.Proxy.Matches, detectedSourceID) || !released.RefundRequired {
		t.Fatal("unban left stale classification or changed finances")
	}
	service.SetDetectedPrefixes([]string{"8.8.8.9/32"})
	subnet := service.EffectiveReport(base, "8.8.8.8")
	if slices.Contains(subnet.Proxy.Matches, detectedSourceID) {
		t.Fatal("a neighboring ban was attributed to requested IP")
	}
}

func TestTransientFirewallEvidenceIsNotWrittenIntoReports(t *testing.T) {
	report := Report{Status: "succeeded", Verdict: "unsuitable", RefundRequired: true, Proxy: &ProxyAssessment{Status: "success", Sources: []ProxySourceResult{{ID: detectedSourceID, Name: "X-Node", Category: AbuseCategory}, {ID: "external", Name: "External", Category: AbuseCategory}}, Matches: []string{detectedSourceID, "external"}}, Refusals: []Refusal{{Kind: "abuse_list", Source: "proxy_lists", Value: "X-Node, External"}}}
	raw, err := EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := DecodeReport(raw)
	if err != nil || slices.Contains(saved.Proxy.Matches, detectedSourceID) || !slices.Contains(saved.Proxy.Matches, "external") || !saved.RefundRequired || len(report.Proxy.Matches) != 2 {
		t.Fatal("durability boundary lost independent evidence or retained live ban")
	}
}

func TestCachedPoolReceivesOnlyCurrentFirewallGeneration(t *testing.T) {
	service := NewEnhancedService(nil, nil, nil)
	pool := &proxyPool{indexes: []*prefixIndex{}, sources: []ProxySourceResult{}, complete: true}
	service.SetDetectedPrefixes([]string{"8.8.8.8/32"})
	first := service.detectedPool(pool.retain())
	defer first.release()
	service.SetDetectedPrefixes(nil)
	second := service.detectedPool(pool.retain())
	defer second.release()
	if len(pool.sources) != 0 || !first.indexes[0].contains(netip.MustParseAddr("8.8.8.8")) || second.indexes[0].contains(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("cached inventory retained a previous firewall generation")
	}
}
