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
	if !checkpoint.LiveDone {
		checkpoint.Phase = "live"
		if err := save(); err != nil {
			return err
		}
		// Enrichment has a short deadline; active probes have their own long budgets.
		liveCtx, cancel := context.WithTimeout(withQualityScores(ctx, checkpoint.Scores), 60*time.Second)
		liveCtx = context.WithValue(liveCtx, deferBlockKey{}, true)
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
	checkpoint.Phase = "probes"
	if err := save(); err != nil {
		return err
	}
	if err := w.Enhanced.probePorts(ctx, r, c, save); err != nil {
		return err
	}
	requested := Aggregate(*r, time.Now().UTC())
	applyEnhancedVerdict(&requested)
	if len(requested.Refusals) == 0 && r.LiveDetails != nil && r.LiveDetails.Block.Status == "skipped" {
		r.LiveDetails.Block = w.Live.block(ctx, r.IP, c)
		if err := save(); err != nil {
			return err
		}
	}
	reason := ""
	if requested.Verdict != "suitable" {
		reason = "ip_not_passed"
	} else if r.Proxy.Status != "success" {
		reason = "proxy_incomplete"
	} else if !probesPassed(r.Ports) || r.LiveDetails == nil || liveMetaFailed(r.LiveDetails.Shodan.LiveMeta) {
		reason = "probes_incomplete"
	}
	if reason != "" {
		finishInventorySubnet(r, reason)
	}
	checkpoint.Phase = "subnet"
	if err := save(); err != nil {
		return err
	}
	if reason == "" {
		if err := w.Enhanced.checkSubnet(ctx, r, c, save); err != nil {
			return err
		}
	}
	applyPortRefusals(r)
	if err := w.Enhanced.probeSubnet(ctx, r, c, save); err != nil {
		return err
	}
	checkpoint.Phase = "complete"
	return save()
}
func applyProxyRefusal(r *Report) {
	if r.Proxy == nil {
		return
	}
	groups := map[string][]string{}
	for _, id := range r.Proxy.Matches {
		name, category := id, ProxyPoolCategory
		for _, source := range r.Proxy.Sources {
			if source.ID == id {
				name, category = source.Name, sourceCategory(source.Category)
				break
			}
		}
		kind := "proxy_list"
		if category == ProxyDatabaseCategory {
			kind = "proxy_database"
		}
		if category == AbuseCategory {
			kind = "abuse_list"
		}
		groups[kind] = append(groups[kind], name)
	}
	for _, kind := range []string{"proxy_list", "proxy_database", "abuse_list"} {
		if len(groups[kind]) == 0 {
			continue
		}
		exists := false
		for _, item := range r.Refusals {
			exists = exists || item.Kind == kind
		}
		if !exists {
			r.Refusals = append(r.Refusals, Refusal{Kind: kind, Source: "proxy_lists", Value: joinEvidence(groups[kind])})
		}
	}
}
func applyEnhancedVerdict(r *Report) {
	moveDatacenterWarnings(r)
	applyProxyRefusal(r)
	applyShodanRefusals(r)
	applyPortRefusals(r)
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
