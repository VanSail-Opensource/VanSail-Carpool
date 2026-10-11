package iplookup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

// reportRetryRepository repairs frozen cache rows and appends durable jobs.
type reportRetryRepository interface {
	IPLookupReportByID(context.Context, string) (Report, Config, error)
	UpdateIPLookupCachedReport(context.Context, Report) error
	EnqueueOutbox(context.Context, string, string, time.Time) error
}

// HandleReportRetryOutbox repairs the failed provider stages of one frozen
// partial report directly in the shared cache row. It never touches receipts
// or billing: members already received the partial verdict, and later polls
// simply observe the improved evidence. The chain policy comes from the
// report's frozen config, matching the credentials that produced it.
func (w *Worker) HandleReportRetryOutbox(ctx context.Context, job model.OutboxJob) error {
	repository, ok := w.repository.(reportRetryRepository)
	if !ok {
		return nil
	}
	var payload ReportRetryPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode %s payload: %w", ReportRetryOutboxKind, err)
	}
	r, c, err := repository.IPLookupReportByID(ctx, payload.ReportID)
	if err != nil {
		if errors.Is(err, ErrRepairGone) {
			return nil
		}
		return err
	}
	unlock := w.lockRun(r.ID)
	defer unlock()
	r, _, err = repository.IPLookupReportByID(ctx, payload.ReportID)
	if err != nil {
		if errors.Is(err, ErrRepairGone) {
			return nil
		}
		return err
	}
	if r.Status != "partial" || r.Checkpoint == nil || !RetryAllowed(c, payload.Attempt) || len(FailedStages(r)) == 0 {
		return nil
	}
	PrepareRepair(&r)
	persist := func(progress Report) error {
		return repository.UpdateIPLookupCachedReport(ctx, progress)
	}
	if err := w.runQueuedStages(ctx, &r, c, persist); err != nil {
		if errors.Is(err, ErrRepairConflict) {
			return nil
		}
		return err
	}
	if err := w.runRPKI(ctx, &r, persist); err != nil {
		if errors.Is(err, ErrRepairConflict) {
			return nil
		}
		return err
	}
	r = Aggregate(r, w.now().UTC())
	if r.Proxy != nil {
		applyEnhancedVerdict(&r)
	}
	if err := repository.UpdateIPLookupCachedReport(ctx, r); err != nil {
		if errors.Is(err, ErrRepairConflict) {
			return nil
		}
		return err
	}
	return w.scheduleNextReportRetry(ctx, repository, r, c, payload.Attempt)
}

// scheduleNextReportRetry continues the chain while the report stays partial
// with failed stages and the frozen policy still allows another attempt.
func (w *Worker) scheduleNextReportRetry(ctx context.Context, repository reportRetryRepository, r Report, c Config, attempt int) error {
	if r.Status != "partial" || len(FailedStages(r)) == 0 || !RetryAllowed(c, attempt+1) {
		return nil
	}
	delay, ok := NextRetryDelay(c)
	if !ok {
		return nil
	}
	payload, err := json.Marshal(ReportRetryPayload{ReportID: r.ID, Attempt: attempt + 1})
	if err != nil {
		return err
	}
	return repository.EnqueueOutbox(ctx, ReportRetryOutboxKind, string(payload), w.now().UTC().Add(delay))
}
