package iplookup

// reconcilePendingStages applies current settings only to unattempted planned work.
// Disabled stages are retained so enabling new providers requires a fresh check.
func reconcilePendingStages(r *Report, c Config) {
	if r.Checkpoint == nil {
		return
	}
	byID := map[string]Stage{}
	completed := []Stage{}
	for _, stage := range r.Checkpoint.Stages {
		if stage.Status == "queued" && !stage.Attempted {
			byID[stage.ID] = stage
		} else {
			completed = append(completed, stage)
		}
	}
	for _, provider := range c.Providers {
		stage, ok := byID[provider.ID]
		if !ok {
			continue
		}
		if !provider.Enabled {
			stage.Status = "disabled"
		}
		completed = append(completed, stage)
		delete(byID, provider.ID)
	}
	r.Checkpoint.Stages = completed
}
