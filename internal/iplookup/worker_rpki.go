package iplookup

import (
	"context"
	"time"
)

// runRPKI is a separate checkpoint: it never changes risk-provider order or billing.
func (w *Worker) runRPKI(ctx context.Context, r *Report, persist func(Report) error) error {
	if r.Checkpoint == nil || r.Checkpoint.RPKI == nil {
		return nil
	}
	stage := r.Checkpoint.RPKI
	if stage.Status == "success" || stage.Status == "error" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	result := RPKIResult{LiveMeta: LiveMeta{Status: "error", Source: "ripe:rpki-validation", ErrorCode: "IP_DETAILS_UNAVAILABLE"}, Routes: []RPKIRoute{}}
	if r.RPKI != nil {
		// An interrupted repair cannot erase previously verified invalid origins.
		result = *r.RPKI
		result.Status, result.ErrorCode = "partial", "IP_DETAILS_UNAVAILABLE"
	}
	if stage.Status != "processing" && w.Live != nil {
		stage.Status = "processing"
		if err := persist(*r); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		attemptCtx = withLiveRequests(attemptCtx)
		memo := attemptCtx.Value(liveRequestsKey{}).(*liveRequests)
		memo.onAttempt = func(context.Context) error { stage.Attempted = true; return persist(*r) }
		result = w.Live.repairRPKI(attemptCtx, r.IP, r.RPKI)
		if err := ctx.Err(); err != nil {
			return err
		}
		memo.mu.Lock()
		attemptErr := memo.attemptErr
		memo.mu.Unlock()
		if attemptErr != nil {
			return attemptErr
		}
	}
	r.RPKI = &result
	stage.Status = "success"
	if liveMetaFailed(result.LiveMeta) {
		stage.Status = "error"
	}
	applyRPKIRefusals(r)
	return persist(*r)
}
