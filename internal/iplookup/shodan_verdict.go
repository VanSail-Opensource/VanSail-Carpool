package iplookup

import "strings"

// applyShodanRefusals keeps every detected tag/CVE in the quality evidence.
// CPEs describe software and do not independently refuse an address.
func applyShodanRefusals(r *Report) {
	if r.LiveDetails == nil {
		return
	}
	seen := map[string]bool{}
	for _, refusal := range r.Refusals {
		if value, ok := refusal.Value.(string); ok && refusal.Source == "shodan:internetdb" {
			seen[refusal.Kind+":"+value] = true
		}
	}
	for _, group := range []struct {
		kind  string
		items []string
	}{{"shodan_tag", r.LiveDetails.Shodan.Tags}, {"shodan_vuln", r.LiveDetails.Shodan.Vulns}} {
		for _, value := range group.items {
			value = strings.TrimSpace(value)
			key := group.kind + ":" + value
			if value != "" && !seen[key] {
				r.Refusals = append(r.Refusals, Refusal{Kind: group.kind, Source: "shodan:internetdb", Value: value})
				seen[key] = true
			}
		}
	}
}
