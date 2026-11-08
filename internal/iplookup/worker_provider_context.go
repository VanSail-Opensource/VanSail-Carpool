package iplookup

import (
	"context"
	"time"
)

func (w *Worker) recordQualityScores(r *Report, result ProviderResult) {
	if w.Live == nil {
		return
	}

	if r.Checkpoint.Enrichment == nil {
		r.Checkpoint.Enrichment = &EnrichmentCheckpoint{}
	}
	if result.ID == "ipapi" && (result.Status == "success" || result.Status == "partial") {
		scores := NetworkScores{LiveMeta: LiveMeta{Status: "success", Source: "ipapi", ObservedAt: w.now().UTC().Format(time.RFC3339)}}
		if result.ContextScores != nil {
			scores.Company, scores.ASN = result.ContextScores.Company, result.ContextScores.ASN
		}
		if v, ok := result.Scores["company_abuse_ratio"]; ok {
			scores.Company.Ratio = &v
		}
		if v, ok := result.Scores["asn_abuse_ratio"]; ok {
			scores.ASN.Ratio = &v
		}
		if scores.Company.Ratio == nil || scores.ASN.Ratio == nil {
			scores.Status = "partial"
		}
		r.Checkpoint.Enrichment.Scores = &scores
	}
}

type qualityScoresKey struct{}

func withQualityScores(ctx context.Context, scores *NetworkScores) context.Context {
	return context.WithValue(ctx, qualityScoresKey{}, scores)
}
