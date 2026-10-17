package iplookup

import (
	"maps"
	"net/netip"
	"slices"
	"time"
)

const detectedSourceID = "xnode-detected"

// withoutDetectedReport removes transient enforcement from durable provider facts.
// It never removes another inventory's evidence or changes financial fields.
func withoutDetectedReport(report Report) Report {
	hadMatch := report.Proxy != nil && slices.Contains(report.Proxy.Matches, detectedSourceID)
	if report.Proxy != nil {
		proxy := *report.Proxy
		proxy.Sources = slices.DeleteFunc(slices.Clone(proxy.Sources), func(source ProxySourceResult) bool { return source.ID == detectedSourceID })
		proxy.Matches = withoutDetectedIDs(proxy.Matches)
		report.Proxy = &proxy
	}
	report.Subnet = withoutDetectedSubnet(report.Subnet)
	if !hadMatch {
		return report
	}
	report.Refusals = slices.DeleteFunc(slices.Clone(report.Refusals), func(refusal Refusal) bool {
		return refusal.Source == "proxy_lists" || refusal.Source == detectedSourceID
	})
	report.Reasons = slices.Clone(report.Reasons)
	report.Warnings = slices.Clone(report.Warnings)
	report.Sources = maps.Clone(report.Sources)
	applyProxyRefusal(&report)
	if report.Checkpoint != nil && (report.Status == "succeeded" || report.Status == "partial") {
		refunded := report.RefundRequired
		checked, _ := time.Parse(time.RFC3339Nano, report.CheckedAt)
		report = Aggregate(report, checked)
		applyEnhancedVerdict(&report)
		report.RefundRequired = refunded
	} else if len(report.Refusals) == 0 && report.Verdict == "unsuitable" {
		// Old terminal payloads did not retain negative provider coverage. An
		// unban cannot manufacture a clean provider result from missing evidence.
		report.Verdict = "inconclusive"
		report.Reasons = []string{"incomplete_coverage"}
	}
	updateRating(&report)
	return report
}

func withoutDetectedIDs(ids []string) []string {
	return slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return id == detectedSourceID })
}

// EffectiveReport overlays current firewall membership on exact or /24 cache
// responses without network calls, new receipts, fees or persisted telemetry.
func (s *EnhancedService) EffectiveReport(report Report, requestedIP string) Report {
	report = withoutDetectedReport(report)
	if requestedIP == "" {
		requestedIP = report.IP
	}
	s.mu.Lock()
	index := s.detected.retain()
	s.mu.Unlock()
	if index == nil {
		return report
	}
	defer index.release()
	if report.Proxy == nil {
		report.Proxy = &ProxyAssessment{Status: "partial", Sources: []ProxySourceResult{}, Matches: []string{}}
	} else {
		copy := *report.Proxy
		copy.Sources = slices.Clone(copy.Sources)
		copy.Matches = slices.Clone(copy.Matches)
		report.Proxy = &copy
	}
	report.Proxy.Sources = append(report.Proxy.Sources, ProxySourceResult{ID: detectedSourceID, Name: "X-Node", Category: AbuseCategory, Status: "success", Count: index.count()})
	ip, err := netip.ParseAddr(requestedIP)
	if err == nil && index.contains(ip.Unmap()) {
		report.Proxy.Matches = append(report.Proxy.Matches, detectedSourceID)
		report.Refusals = slices.Clone(report.Refusals)
		report.Refusals = append(report.Refusals, Refusal{Kind: "abuse_list", Source: detectedSourceID, Value: "X-Node"})
		report.Verdict = "unsuitable"
		report.Reasons = append(slices.Clone(report.Reasons), detectedSourceID+":abuse_list")
	}
	if report.Subnet != nil {
		report.Subnet.Sources = append(report.Subnet.Sources, InventorySource{ID: detectedSourceID, Name: "X-Node", Category: AbuseCategory})
		for i := range report.Subnet.Addresses {
			row := &report.Subnet.Addresses[i]
			address, err := netip.ParseAddr(row.IP)
			if err == nil && index.contains(address.Unmap()) {
				row.Sources = append(slices.Clone(row.Sources), detectedSourceID)
				row.Status, row.Abuse = "flagged", "flagged"
			}
		}
	}
	report.Warnings = slices.Clone(report.Warnings)
	updateRating(&report)
	return report
}
