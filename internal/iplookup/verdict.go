package iplookup

import (
	"strings"
	"time"
)

// NewReport reserves the configured stages without storing provider payloads.
func NewReport(id, ip string, c Config) Report {
	r := Report{ID: id, IP: ip, Status: "processing", Verdict: "inconclusive", Reasons: []string{}, Sources: map[string]string{}, Databases: []Database{}, Refusals: []Refusal{}, PolicyVersion: Version, ParserVersion: Version, Checkpoint: newCheckpoint()}
	for _, p := range c.Providers {
		status := "disabled"
		if p.Enabled {
			status = "queued"
		}
		r.Checkpoint.Stages = append(r.Checkpoint.Stages, Stage{ID: p.ID, Status: status})
	}
	return r
}

func newCheckpoint() *Checkpoint {
	return &Checkpoint{Stages: []Stage{}, Complete: true, Votes: map[string][]string{}}
}

// RecordProvider completes a stage once and folds its transient normalized evidence.
func RecordProvider(r *Report, p ProviderResult, c Config) {
	foldLegacy(r, c)
	if r.Checkpoint == nil {
		r.Checkpoint = newCheckpoint()
	}
	for i := range r.Checkpoint.Stages {
		stage := &r.Checkpoint.Stages[i]
		if stage.ID != p.ID {
			continue
		}
		if stage.Status == "success" || stage.Status == "partial" || stage.Status == "error" {
			return
		}
		stage.Status, stage.Attempted, stage.Complete = p.Status, true, p.Complete
		applyProvider(r, p, c)
		syncFacts(r)
		return
	}
	r.Checkpoint.Stages = append(r.Checkpoint.Stages, Stage{ID: p.ID, Status: p.Status, Attempted: true})
	applyProvider(r, p, c)
	syncFacts(r)
}

// Aggregate completes the compact verdict; contacted API errors are inconclusive.
func Aggregate(r Report, now time.Time) Report {
	foldLegacy(&r, DefaultConfig())
	if r.Checkpoint == nil {
		return r
	}
	syncFacts(&r)
	r.RefundRequired = false
	r.CheckedAt = now.UTC().Format(time.RFC3339Nano)
	r.Status, r.Verdict = "succeeded", "inconclusive"
	r.Reasons = []string{}
	complete := r.Checkpoint.Complete && r.Checkpoint.Coverage == 31
	if !r.Checkpoint.Complete || r.Checkpoint.Usable == 0 {
		r.Status = "partial"
	}
	if len(r.Refusals) > 0 {
		r.Verdict = "unsuitable"
		for _, item := range r.Refusals {
			r.Reasons = append(r.Reasons, item.Source+":"+item.Kind)
		}
	} else {
		if !complete {
			r.Status = "partial"
			r.Reasons = append(r.Reasons, "incomplete_coverage")
		}
		eligible := r.Facts.NetworkType == "residential" || r.Facts.NetworkType == "mobile"
		if !eligible {
			r.Reasons = append(r.Reasons, "residential_unknown")
		}
		if complete && eligible {
			r.Verdict = "suitable"
		}
	}
	// The checkpoint survives aggregation so background repair of partial
	// reports can resume folding exactly where the original run stopped.
	r.Providers = nil
	return r
}

func applyProvider(r *Report, p ProviderResult, c Config) {
	if r.Checkpoint == nil {
		r.Checkpoint = newCheckpoint()
	}
	state := r.Checkpoint
	if p.Status != "success" && p.Status != "partial" {
		state.Complete = false
		return
	}
	state.Usable++
	state.Complete = state.Complete && p.Status == "success" && p.Complete
	for i, v := range []*bool{p.Signals.Abuse, p.Signals.Datacenter, p.Signals.VPN, p.Signals.Proxy, p.Signals.Tor} {
		if v != nil {
			state.Coverage |= 1 << i
		}
	}
	if kind := p.Facts.NetworkType; kind != "" {
		state.Votes[kind] = append(state.Votes[kind], source(p, "networkType"))
	}
	r.Refusals = append(r.Refusals, providerRefusals(p)...)
	if p.ID == "maxmind" {
		r.MaxMind = p.MaxMind
	}
	if r.Facts.ASN == "" && p.Facts.ASN != "" {
		r.Facts.ASN, r.Facts.ASNName = p.Facts.ASN, p.Facts.ASNName
		r.Sources["asn"] = source(p, "asn")
		if p.Facts.ASNName != "" {
			r.Sources["asnName"] = source(p, "asnName")
		}
	}
	if r.Facts.ASN != "" && r.Facts.ASN == p.Facts.ASN && r.Facts.ASNName == "" && p.Facts.ASNName != "" {
		r.Facts.ASNName = p.Facts.ASNName
		r.Sources["asnName"] = source(p, "asnName")
	}
	if geo := p.Geo; geo != nil && betterGeo(geo, state.Geo, c.GeolocationOrder) {
		state.Geo = geo
	}
	syncFacts(r)
}

func syncFacts(r *Report) {
	state := r.Checkpoint
	if state == nil {
		return
	}
	r.Databases = []Database{}
	for _, stage := range state.Stages {
		if stage.Attempted {
			r.Databases = append(r.Databases, Database{ID: stage.ID, Status: stage.Status})
		}
	}
	r.Facts.NetworkType = ""
	delete(r.Sources, "networkType")
	total := 0
	for _, voters := range state.Votes {
		total += len(voters)
	}
	for kind, voters := range state.Votes {
		if len(voters)*2 > total {
			r.Facts.NetworkType = kind
			r.Sources["networkType"] = strings.Join(voters, ",")
		}
	}
	if geo := state.Geo; geo != nil {
		r.Facts.Country, r.Facts.City = geo.Country, geo.City
		r.Facts.Latitude, r.Facts.Longitude = geo.Latitude, geo.Longitude
		for key, present := range map[string]bool{"country": geo.Country != "", "city": geo.City != "", "latitude": geo.Latitude != nil, "longitude": geo.Longitude != nil} {
			delete(r.Sources, key)
			if present {
				r.Sources[key] = geo.Source
			}
		}
	}
	for key, present := range map[string]bool{"ip_risk_snapshot": r.MaxMind.IPRiskSnapshot != nil, "static_ip_score": r.MaxMind.StaticIPScore != nil, "user_count": r.MaxMind.UserCount != nil, "user_type": r.MaxMind.UserType != nil} {
		if present {
			r.Sources["maxmind."+key] = "maxmind"
		}
	}
}

func source(p ProviderResult, key string) string {
	if p.Sources[key] != "" {
		return p.Sources[key]
	}
	return p.ID
}
