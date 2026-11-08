package iplookup

import (
	"context"
)

// EnrichmentCheckpoint retains read-only enrichment recovery and provider scores.
// The legacy JSON key is kept so interrupted pre-retirement lookups can resume.
type EnrichmentCheckpoint struct {
	LiveDone bool           `json:"liveDone"`
	Scores   *NetworkScores `json:"scores,omitempty"`
}

// SharedProgress exposes only the requested address and public stage progress.
type SharedProgress struct {
	IP        string    `json:"ip"`
	Status    string    `json:"status"`
	Progress  *Progress `json:"progress"`
	UpdatedAt string    `json:"updatedAt"`
}

func (w *Worker) runEnrichment(ctx context.Context, r *Report, c Config) error {
	if r.Checkpoint.Enrichment == nil {
		r.Checkpoint.Enrichment = &EnrichmentCheckpoint{}
	}
	stage := r.Checkpoint.Enrichment
	if stage.LiveDone {
		return nil
	}
	if err := w.refreshLive(ctx, r, c); err != nil {
		return err
	}
	stage.LiveDone = true
	return w.repository.SaveIPLookupProgress(ctx, *r)
}

func applyEnrichmentVerdict(r *Report) {
	moveDatacenterWarnings(r)
	applyShodanRefusals(r)
	if len(r.Refusals) > 0 {
		r.Verdict, r.Reasons = "unsuitable", []string{}
		for _, item := range r.Refusals {
			r.Reasons = append(r.Reasons, item.Source+":"+item.Kind)
		}
	}
	if r.LiveDetails != nil && LivePartial(*r.LiveDetails) {
		r.Status = "partial"
	}
	updateRating(r)
}
