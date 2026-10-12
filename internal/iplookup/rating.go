package iplookup

import (
	"net/netip"
	"strings"
)

func eligibleNetwork(r Report) bool {
	return r.Facts.NetworkType == "residential" || r.Facts.NetworkType == "mobile" || r.Facts.NetworkType == "business" || r.Facts.NetworkType == "datacenter" || hasDatacenterWarning(r) || businessUsage(r)
}

func businessUsage(r Report) bool {
	return r.Facts.NetworkType == "business" || (r.MaxMind.UserType != nil && networkType(strings.TrimSpace(*r.MaxMind.UserType)) == "business")
}

func coreQualityPassed(r Report) bool {
	complete := r.Verdict == "suitable" && len(r.Databases) > 0
	for _, database := range r.Databases {
		complete = complete && database.Status == "success"
	}
	if state := r.Checkpoint; state != nil {
		complete = state.Complete && state.Coverage == 31 && state.Usable > 0 && eligibleNetwork(r)
	}
	if !complete || r.Proxy == nil || r.Proxy.Status != "success" || r.RPKI == nil || r.LiveDetails == nil {
		return false
	}
	if !qualitySourcePassed(r.RPKI.Status) || !qualitySourcePassed(r.LiveDetails.Shodan.Status) {
		return false
	}
	for _, route := range r.RPKI.Routes {
		if !qualitySourcePassed(route.Status) || route.Validity == "unavailable" {
			return false
		}
	}
	return true
}

func qualitySourcePassed(status string) bool { return status == "success" || status == "empty" }

func updateRating(r *Report) {
	moveDatacenterWarnings(r)
	r.Rating = "incomplete"
	if len(r.Refusals) > 0 || (r.Proxy != nil && len(r.Proxy.Matches) > 0) {
		r.Rating = "poor"
		return
	}
	if r.RPKI != nil {
		for _, route := range r.RPKI.Routes {
			if route.Status == "success" && (route.Validity == "invalid_asn" || route.Validity == "invalid_length") {
				r.Rating = "poor"
				return
			}
		}
	}
	for _, port := range r.Ports {
		for _, finding := range port.Findings {
			if finding.Confidence == "confirmed" {
				r.Rating = "poor"
				return
			}
		}
	}
	if r.LiveDetails != nil && (len(r.LiveDetails.Shodan.Tags) > 0 || len(r.LiveDetails.Shodan.Vulns) > 0) {
		r.Rating = "poor"
		return
	}
	if !coreQualityPassed(*r) {
		return
	}
	if hasDatacenterWarning(*r) || r.Facts.NetworkType == "datacenter" {
		r.Rating = "average"
		return
	}
	if businessUsage(*r) || !probesPassed(r.Ports) || observedPorts(r.Ports) {
		r.Rating = "good"
		return
	}
	r.Rating = "excellent"
	if subnetPassed(*r) {
		r.Rating = "perfect"
	}
}

func observedPorts(ports []PortProbe) bool {
	for _, port := range ports {
		if port.Status == "closed" {
			continue
		}
		for _, task := range port.Tasks {
			if task.Key == "tcp" && task.Status == "success" {
				return true
			}
		}
		for _, finding := range port.Findings {
			if finding.Confidence == "observed" || finding.Confidence == "suspected" {
				return true
			}
		}
	}
	return false
}

func subnetPassed(r Report) bool {
	if ip, err := netip.ParseAddr(r.IP); err == nil && ip.Is6() {
		return true
	}
	if r.Subnet == nil || r.Subnet.Status != "success" {
		return false
	}
	neighbors := 0
	for _, row := range r.Subnet.Addresses {
		if row.IP == r.IP {
			continue
		}
		neighbors++
		if row.Status != "clean" {
			return false
		}
	}
	return neighbors == 255
}
