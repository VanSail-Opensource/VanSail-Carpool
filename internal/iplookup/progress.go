package iplookup

// Progress describes completed work, not estimated time or successful sources.
type Progress struct {
	Phase           string `json:"phase"`
	Completed       int    `json:"completed"`
	Total           int    `json:"total"`
	CurrentProvider string `json:"currentProvider,omitempty"`
}

// CheckProgress projects private checkpoints without exposing unfinished verdicts.
func CheckProgress(r Report) *Progress {
	p := &Progress{Phase: "checking"}
	if r.Status != "processing" {
		p.Phase, p.Completed, p.Total = "complete", 1, 1
		return p
	}
	if r.Checkpoint == nil {
		p.Phase = "queued"
		return p
	}
	for _, stage := range r.Checkpoint.Stages {
		if stage.Status == "disabled" {
			continue
		}
		p.Total++
		switch stage.Status {
		case "success", "partial", "error", "skipped":
			p.Completed++
		case "processing":
			p.CurrentProvider = stage.ID
		}
	}
	if p.CurrentProvider == "" && p.Completed == 0 {
		p.Phase = "queued"
	}
	return p
}
