package iplookup

import "encoding/json"

// EncodeReport persists only the compact projection and minimal recovery state.
// Partial reports keep their checkpoint because they stay eligible for
// background repair; terminal reports drop it.
func EncodeReport(r Report) ([]byte, error) {
	foldLegacy(&r, DefaultConfig())
	if r.Status != "processing" && r.Status != "partial" {
		r.Checkpoint = nil
	}
	return json.Marshal(struct {
		Report
		Checkpoint *Checkpoint `json:"_checkpoint,omitempty"`
	}{Report: PublicReport(r), Checkpoint: r.Checkpoint})
}

// DecodeReport compacts legacy payloads while preserving their frozen outcomes.
func DecodeReport(raw []byte) (Report, error) {
	var saved struct {
		Report
		Checkpoint *Checkpoint      `json:"_checkpoint"`
		Providers  []ProviderResult `json:"providers"`
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		return Report{}, err
	}
	r := saved.Report
	r.Checkpoint, r.Providers = saved.Checkpoint, saved.Providers
	foldLegacy(&r, DefaultConfig())
	if r.Status != "processing" && r.Status != "partial" {
		r.Checkpoint = nil
	}
	normalizeReport(&r)
	return r, nil
}

// PublicReport removes private recovery metadata from member responses.
func PublicReport(r Report) Report {
	if r.Rating == "" {
		updateRating(&r)
	}
	r.Checkpoint, r.Providers = nil, nil
	normalizeReport(&r)
	if r.LiveDetails != nil {
		details := CompactLiveDetails(*r.LiveDetails)
		r.LiveDetails = &details
	}
	return r
}

// ReportAttempted distinguishes contacted lookups from unattempted queue recovery.
func ReportAttempted(r Report) bool {
	if r.Checkpoint != nil {
		for _, stage := range r.Checkpoint.Stages {
			if stage.Attempted {
				return true
			}
		}
	}
	return len(r.Databases) > 0
}

func normalizeReport(r *Report) {
	moveDatacenterWarnings(r)
	if r.Proxy != nil {
		proxy := *r.Proxy
		proxy.Sources = append([]ProxySourceResult{}, proxy.Sources...)
		for i := range proxy.Sources {
			proxy.Sources[i].Category = sourceCategory(proxy.Sources[i].Category)
		}
		r.Proxy = &proxy
	}
	if r.Sources == nil {
		r.Sources = map[string]string{}
	}
	delete(r.Sources, "region")
	delete(r.Sources, "isp")
	if r.Reasons == nil {
		r.Reasons = []string{}
	}
	if r.Databases == nil {
		r.Databases = []Database{}
	}
	if r.Refusals == nil {
		r.Refusals = []Refusal{}
	}
	if r.Checkpoint != nil && r.Checkpoint.Votes == nil {
		r.Checkpoint.Votes = map[string][]string{}
	}
}

func foldLegacy(r *Report, c Config) {
	normalizeReport(r)
	if len(r.Providers) == 0 {
		return
	}
	legacy := r.Providers
	r.Providers = nil
	r.Checkpoint = newCheckpoint()
	r.Refusals = []Refusal{}
	for _, p := range legacy {
		attempted := p.Status == "processing" || p.Status == "success" || p.Status == "partial" || p.Status == "error"
		r.Checkpoint.Stages = append(r.Checkpoint.Stages, Stage{ID: p.ID, Status: p.Status, Attempted: attempted})
		if p.Geo == nil && (p.Facts.Country != "" || p.Facts.City != "") {
			p.Geo = &GeoCandidate{Country: p.Facts.Country, City: p.Facts.City, Source: p.ID, Provider: p.ID}
		}
		if p.ID == "maxmind" {
			if v, ok := p.Scores["static_ip_score"]; ok {
				p.MaxMind.StaticIPScore = &v
			}
			if v, ok := p.Scores["network_risk_snapshot"]; ok {
				p.MaxMind.IPRiskSnapshot = &v
			}
		}
		if p.Status == "success" || p.Status == "partial" || p.Status == "error" {
			applyProvider(r, p, c)
		}
	}
	syncFacts(r)
}
