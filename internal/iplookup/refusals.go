package iplookup

// RiskReasons supplies stable identifiers for the shared stop-on-first-refusal policy.
func RiskReasons(p ProviderResult) []string {
	reasons := []string{}
	for _, item := range providerRefusals(p) {
		reasons = append(reasons, item.Source+":"+item.Kind)
	}
	return reasons
}

func providerRefusals(p ProviderResult) []Refusal {
	items := []Refusal{}
	for _, flag := range []struct {
		kind  string
		value *bool
	}{{"abuse", p.Signals.Abuse}, {"vpn", p.Signals.VPN}, {"proxy", p.Signals.Proxy}, {"tor", p.Signals.Tor}} {
		if flag.value == nil || !*flag.value {
			continue
		}
		var evidence any = true
		if detail := p.Details[flag.kind]; detail != "" {
			evidence = detail
		}
		items = append(items, Refusal{Kind: flag.kind, Source: source(p, flag.kind), Value: evidence})
	}
	if p.Reports != nil && *p.Reports > 0 {
		items = append(items, Refusal{Kind: "abuse_reports", Source: p.ID, Value: *p.Reports})
	}
	for _, key := range []string{"abuse_confidence", "fraud_score"} {
		if key == "fraud_score" && p.RiskLevel == "low" {
			continue
		}
		if value := p.Scores[key]; value > 0 {
			items = append(items, Refusal{Kind: key, Source: p.ID, Value: value})
		}
	}
	if p.Scores["fraud_score"] <= 0 && (p.RiskLevel == "medium" || p.RiskLevel == "high" || p.RiskLevel == "very_high") {
		items = append(items, Refusal{Kind: "fraud_score", Source: p.ID, Value: p.RiskLevel})
	}
	if p.MaxMind.UserCount != nil && *p.MaxMind.UserCount > 5 {
		items = append(items, Refusal{Kind: "user_count", Source: "maxmind", Value: *p.MaxMind.UserCount})
	}
	return items
}
