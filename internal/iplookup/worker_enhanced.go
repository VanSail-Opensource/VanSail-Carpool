package iplookup

import (
	"context"
	"time"
)

func (w *Worker) runEnhanced(ctx context.Context, r *Report, c Config) error {
	if r.Checkpoint.Enhanced == nil {
		r.Checkpoint.Enhanced = &EnhancedCheckpoint{Phase: "checking"}
	}
	checkpoint := r.Checkpoint.Enhanced
	save := func() error { return w.repository.SaveIPLookupProgress(ctx, *r) }
	if r.Proxy == nil {
		checkpoint.Phase = "proxy"
		if err := save(); err != nil {
			return err
		}
		// Global inventory membership does not depend on optional geolocation.
		pool, err := w.Enhanced.pool(ctx, c)
		if err != nil {
			return err
		}
		r.Proxy = pool.assessment(r.IP)
		applyProxyRefusal(r)
	}
	if !checkpoint.LiveDone {
		checkpoint.Phase = "live"
		if err := save(); err != nil {
			return err
		}
		// Enrichment has a short deadline; active probes have their own long budgets.
		liveCtx, cancel := context.WithTimeout(withQualityScores(ctx, checkpoint.Scores), 60*time.Second)
		details := w.Live.livePipeline(withLiveRequests(liveCtx), r.IP, c, func(LiveEvent) {})
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		details.SourceIP, details.CacheMatch = r.IP, "none"
		r.LiveDetails = &details
		checkpoint.LiveDone = true
		if err := save(); err != nil {
			return err
		}
	}
	if checkpoint.LiveDone && r.LiveDetails != nil && r.LiveDetails.Block.Status == "skipped" {
		r.LiveDetails.Block = w.Live.block(ctx, r.IP, c)
		if err := save(); err != nil {
			return err
		}
	}
	checkpoint.Phase = "probes"
	if err := save(); err != nil {
		return err
	}
	if err := w.Enhanced.probePorts(ctx, r, c, save); err != nil {
		return err
	}
	requested := Aggregate(*r, time.Now().UTC())
	applyEnhancedVerdict(&requested)
	reason := ""
	if requested.Verdict != "suitable" {
		reason = "ip_not_passed"
	} else if r.Proxy.Status != "success" {
		reason = "proxy_incomplete"
	} else if !probesPassed(r.Ports) || r.LiveDetails == nil || liveMetaFailed(r.LiveDetails.Shodan.LiveMeta) {
		reason = "probes_incomplete"
	}
	if reason != "" {
		r.Subnet = &SubnetResult{Prefix: blockPrefix(r.IP), Status: "skipped", Reason: reason, Addresses: []SubnetAddress{}}
		checkpoint.Phase = "complete"
		return save()
	}
	checkpoint.Phase = "subnet"
	if err := save(); err != nil {
		return err
	}
	if err := w.Enhanced.checkSubnet(ctx, r, c, save); err != nil {
		return err
	}
	checkpoint.Phase = "complete"
	return save()
}
func applyProxyRefusal(r *Report) {
	if r.Proxy != nil && len(r.Proxy.Matches) > 0 {
		for _, item := range r.Refusals {
			if item.Kind == "proxy_list" {
				return
			}
		}
		names := []string{}
		for _, id := range r.Proxy.Matches {
			name := id
			for _, source := range r.Proxy.Sources {
				if source.ID == id {
					name = source.Name
					break
				}
			}
			names = append(names, name)
		}
		r.Refusals = append(r.Refusals, Refusal{Kind: "proxy_list", Source: "proxy_lists", Value: joinEvidence(names)})
	}
}
func applyEnhancedVerdict(r *Report) {
	moveDatacenterWarnings(r)
	applyProxyRefusal(r)
	applyShodanRefusals(r)
	for _, port := range r.Ports {
		for _, finding := range port.Findings {
			if finding.Confidence == "confirmed" {
				exists := false
				for _, refusal := range r.Refusals {
					exists = exists || (refusal.Kind == "active_proxy" && refusal.Source == finding.Protocol)
				}
				if !exists {
					r.Refusals = append(r.Refusals, Refusal{Kind: "active_proxy", Source: finding.Protocol, Value: port.Port})
				}
			}
		}
	}
	if len(r.Refusals) > 0 {
		r.Verdict = "unsuitable"
		r.Reasons = []string{}
		for _, v := range r.Refusals {
			r.Reasons = append(r.Reasons, v.Source+":"+v.Kind)
		}
	}
	if (r.Proxy != nil && r.Proxy.Status != "success") || !probesPassed(r.Ports) {
		r.Status = "partial"
		if r.Verdict == "suitable" {
			r.Verdict = "inconclusive"
		}
	}
	if r.LiveDetails != nil && LivePartial(*r.LiveDetails) {
		r.Status = "partial"
	}
	if r.Subnet != nil && r.Subnet.Status == "partial" {
		r.Status = "partial"
	}
	updateRating(r)
}
// probesPassed treats unavailable probes as passing: a port whose probe
// errored (for example filtered or not open from the prober) carries no
// adverse evidence, so it must not degrade the verdict, status, or rating.
// Only still-running probes or confirmed proxy findings fail the gate.
func probesPassed(ports []PortProbe) bool {
	if len(ports) == 0 {
		return false
	}
	for _, p := range ports {
		if p.Status != "success" && p.Status != "closed" && p.Status != "error" {
			return false
		}
		for _, f := range p.Findings {
			if f.Confidence == "confirmed" {
				return false
			}
		}
	}
	return true
}
