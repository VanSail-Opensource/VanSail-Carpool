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

func (w *Worker) refreshLive(ctx context.Context, r Report, c Config) error {
	repo, ok := w.repository.(LiveWorkerRepository)
	if !ok {
		return nil
	}
	stage, claimed, err := repo.ClaimIPLookupLiveStage(ctx, r.ID)
	if err != nil || stage.Status == "" || stage.Status == "succeeded" || stage.Status == "partial" || stage.Status == "error" {
		return err
	}
	if !claimed || w.Live == nil {
		stage.Status = "error"
		return repo.FinishIPLookupLiveStage(ctx, r.ID, stage)
	}
	details, err := w.Live.RefreshRun(ctx, r.IP, c, r.ID, func(callCtx context.Context) error { return repo.MarkIPLookupLiveAttempt(callCtx, r.ID) })
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
	return repo.FinishIPLookupLiveStage(ctx, r.ID, stage)
}
