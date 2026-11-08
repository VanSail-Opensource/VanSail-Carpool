package iplookup

import "context"

// LiveStage is private durable recovery state shared by joined quality checks.
type LiveStage struct {
	Status     string `json:"status"`
	Attempted  bool   `json:"attempted"`
	SnapshotID string `json:"snapshotId"`
}

// LiveWorkerRepository claims the refresh phase and checkpoints its evidence reference.
type LiveWorkerRepository interface {
	ClaimIPLookupLiveStage(context.Context, string) (LiveStage, bool, error)
	MarkIPLookupLiveAttempt(context.Context, string) error
	FinishIPLookupLiveStage(context.Context, string, LiveStage) error
}

func (w *Worker) refreshLive(ctx context.Context, r *Report, c Config) error {
	repo, ok := w.repository.(LiveWorkerRepository)
	if !ok {
		return nil
	}
	stage, claimed, err := repo.ClaimIPLookupLiveStage(ctx, r.ID)
	if err != nil {
		return err
	}
	if stage.Status == "succeeded" || stage.Status == "partial" {
		if stage.SnapshotID != "" && r.LiveDetails == nil {
			if reader, ok := w.repository.(interface {
				IPLiveSnapshotByID(context.Context, string) (LiveDetails, string, error)
			}); ok {
				details, _, err := reader.IPLiveSnapshotByID(ctx, stage.SnapshotID)
				if err != nil {
					return err
				}
				r.LiveDetails = &details
			}
		}
		return nil
	}
	if stage.Status == "" || stage.Status == "error" {
		return nil
	}
	if !claimed || w.Live == nil {
		stage.Status = "error"
		return repo.FinishIPLookupLiveStage(ctx, r.ID, stage)
	}
	details, err := w.Live.RefreshRun(withQualityScores(ctx, r.Checkpoint.Enrichment.Scores), r.IP, c, r.ID, func(callCtx context.Context) error { return repo.MarkIPLookupLiveAttempt(callCtx, r.ID) })
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Storage errors retry checkpoint recovery; source failures are already section data.
	if err != nil {
		return err
	}
	stage.SnapshotID, stage.Status = details.SnapshotID, "succeeded"
	if stage.SnapshotID == "" {
		stage.Status = "error"
	} else if LivePartial(details) {
		stage.Status = "partial"
	}
	if stage.SnapshotID != "" {
		r.LiveDetails = &details
	}
	return repo.FinishIPLookupLiveStage(ctx, r.ID, stage)
}
