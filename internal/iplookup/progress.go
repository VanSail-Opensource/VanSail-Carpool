package iplookup

// ProgressTask is one observable unit of a lookup: a live section, a reputation
// provider stage, or the realtime refresh phase appended to paid checks.
type ProgressTask struct {
	Key       string `json:"key"`
	Status    string `json:"status"` // pending, active, success, partial, error, skipped
	ErrorCode string `json:"errorCode,omitempty"`
}

// Progress describes completed work, not estimated time or successful sources.
type Progress struct {
	Phase           string         `json:"phase"`
	Completed       int            `json:"completed"`
	Total           int            `json:"total"`
	CurrentProvider string         `json:"currentProvider,omitempty"`
	Tasks           []ProgressTask `json:"tasks,omitempty"`
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
	p.Tasks = []ProgressTask{}
	for _, stage := range r.Checkpoint.Stages {
		if stage.Status == "disabled" {
			continue
		}
		p.Total++
		task := ProgressTask{Key: stage.ID, Status: stage.Status}
		switch stage.Status {
		case "queued":
			task.Status = "pending"
		case "processing":
			task.Status = "active"
			p.CurrentProvider = stage.ID
		case "success", "partial", "error", "skipped":
			p.Completed++
		}
		p.Tasks = append(p.Tasks, task)
	}
	if p.CurrentProvider == "" && p.Completed == 0 {
		p.Phase = "queued"
	}
	if stage := r.Checkpoint.RPKI; stage != nil {
		task := ProgressTask{Key: "rpki", Status: stage.Status}
		p.Total++
		switch stage.Status {
		case "queued":
			task.Status = "pending"
		case "processing":
			task.Status = "active"
			p.Phase = "rpki"
		case "success", "error":
			p.Completed++
		}
		p.Tasks = append(p.Tasks, task)
	}
	return p
}
