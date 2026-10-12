package iplookup

import (
	"fmt"
	"strconv"
	"strings"
)

// applyPortRefusals retains every confirmed port and combines its protocols once.
func applyPortRefusals(r *Report) {
	for _, port := range r.Ports {
		protocols := []string{}
		cloudflare := false
		for _, finding := range port.Findings {
			if finding.Confidence != "confirmed" {
				continue
			}
			if finding.Protocol == "cloudflare" {
				cloudflare = true
				continue
			}
			seen := false
			for _, protocol := range protocols {
				seen = seen || protocol == finding.Protocol
			}
			if !seen {
				protocols = append(protocols, finding.Protocol)
			}
		}
		if len(protocols) > 0 {
			upsertPortRefusal(r, "active_proxy", strings.Join(protocols, ","), port.Port)
		}
		if cloudflare {
			upsertPortRefusal(r, "cloudflare_cdn", "cloudflare", port.Port)
		}
	}
}

func upsertPortRefusal(r *Report, kind, source string, port int) {
	kept := make([]Refusal, 0, len(r.Refusals)+1)
	added := false
	for _, item := range r.Refusals {
		if item.Kind == kind && fmt.Sprint(item.Value) == strconv.Itoa(port) {
			if !added {
				kept = append(kept, Refusal{Kind: kind, Source: source, Value: port})
				added = true
			}
			continue
		}
		kept = append(kept, item)
	}
	if !added {
		kept = append(kept, Refusal{Kind: kind, Source: source, Value: port})
	}
	r.Refusals = kept
}
