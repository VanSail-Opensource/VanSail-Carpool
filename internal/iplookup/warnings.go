package iplookup

import "encoding/json"

func providerWarnings(p ProviderResult) []Refusal {
	if (p.Signals.Datacenter == nil || !*p.Signals.Datacenter) && p.Facts.NetworkType != "datacenter" {
		return nil
	}
	var value any = true
	if detail := p.Details["datacenter"]; detail != "" {
		value = detail
	}
	return []Refusal{{Kind: "datacenter", Source: source(p, "datacenter"), Value: value}}
}

func appendWarning(existing []Refusal, items ...Refusal) []Refusal {
	result := append([]Refusal{}, existing...)
	for _, item := range items {
		raw, _ := json.Marshal(item)
		found := false
		for _, previous := range result {
			value, _ := json.Marshal(previous)
			if string(value) == string(raw) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, item)
		}
	}
	return result
}

func moveDatacenterWarnings(r *Report) {
	kept := make([]Refusal, 0, len(r.Refusals))
	for _, item := range r.Refusals {
		if item.Kind == "datacenter" {
			r.Warnings = appendWarning(r.Warnings, item)
		} else {
			kept = append(kept, item)
		}
	}
	r.Refusals = kept
}

func hasDatacenterWarning(r Report) bool {
	for _, item := range r.Warnings {
		if item.Kind == "datacenter" {
			return true
		}
	}
	return false
}
