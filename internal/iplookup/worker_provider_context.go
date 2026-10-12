package iplookup

import "time"

func (w *Worker) recordEnhancedContext(r *Report, result ProviderResult) {
	if w.Enhanced == nil {
		return
	}

	if r.Checkpoint.Enhanced == nil {
		r.Checkpoint.Enhanced = &EnhancedCheckpoint{Phase: "checking"}
	}
	if result.ID == "ipapi" && (result.Status == "success" || result.Status == "partial") {
		r.Checkpoint.Enhanced.IPAPI = &ProviderResult{ID: "ipapi", Status: result.Status, Signals: result.Signals, Complete: result.Complete}
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
		r.Checkpoint.Enhanced.Scores = &scores
	}
}
