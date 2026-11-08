package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

// PurgeJob is handled by the existing durable outbox and upstream executors.
const PurgeJob = "account_purge"

// PurgeRepository retains accounting while removing current account material.
type PurgeRepository interface {
	AccountPurge(context.Context, string, int64) (*model.AccountPurge, error)
	AccountCleanupAdmissionValid(context.Context, string, int64, time.Time) (bool, error)
	AccountCleanupAdmissionReason(context.Context, string, int64, time.Time) (string, error)
	AccountPurgeDependency(context.Context, string, int64, time.Time) (string, error)
	FenceAccountPurge(context.Context, string, int64, time.Time) error
	CompleteAccountPurge(context.Context, string, int64, time.Time) error
}

// PurgeWorker closes native journals before completing local account cleanup.
type PurgeWorker struct {
	Repository PurgeRepository
	Native     interface {
		CloseAccountPeriods(context.Context, model.AccountPurge, string) error
	}
	DeleteEmby func(context.Context, string) error
}

// HandleOutbox resumes only the generation selected by its durable job.
func (w *PurgeWorker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		UserID     string `json:"userId"`
		Generation int64  `json:"generation"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.UserID == "" || input.Generation < 1 || job.ID == "" {
		return errors.New("invalid account purge target")
	}
	target, err := w.Repository.AccountPurge(ctx, input.UserID, input.Generation)
	if err != nil {
		if target != nil {
			if admissionErr := w.guard(ctx, target); admissionErr != nil {
				return admissionErr
			}
		}
		return &outbox.WaitError{Reason: "account_purge_admission_pending", Cause: err, After: time.Minute}
	}
	if target == nil {
		return nil
	}
	if err = w.guard(ctx, target); err != nil {
		return err
	}
	if err = w.Repository.FenceAccountPurge(ctx, target.UserID, target.Generation, time.Now().UTC()); err != nil {
		return &outbox.WaitError{Reason: "account_purge_intent_fence_pending", Cause: err, After: time.Minute}
	}
	if target.Busy {
		return &outbox.WaitError{Reason: "account_purge_mutations_pending", After: time.Minute}
	}
	if target.EmbyUncertain {
		return &outbox.ReviewError{Reason: "account purge has an uncertain Emby creation"}
	}
	dependency, err := w.Repository.AccountPurgeDependency(ctx, target.UserID, target.Generation, time.Now().UTC())
	if err != nil || dependency != "" {
		if dependency == "" {
			dependency = "account_purge_dependency_read_pending"
		}
		return &outbox.WaitError{Reason: dependency, Cause: err, After: time.Minute}
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if target.RemoteEmbyID != "" {
		if err = w.guard(bounded, target); err != nil {
			return err
		}
		remoteCtx, remoteCancel := context.WithTimeout(bounded, 3*time.Second)
		err = w.DeleteEmby(remoteCtx, target.RemoteEmbyID)
		remoteCancel()
		if err != nil {
			return &outbox.WaitError{Reason: "account_purge_emby_delete_pending", Cause: err, After: time.Minute}
		}
	}
	if err = w.guard(bounded, target); err != nil {
		return err
	}
	if err := w.Native.CloseAccountPeriods(bounded, *target, job.ID); err != nil {
		return &outbox.WaitError{Reason: "account_purge_accounting_pending", Cause: err, After: time.Minute}
	}
	if err = w.guard(bounded, target); err != nil {
		return err
	}
	if err = w.Repository.CompleteAccountPurge(bounded, target.UserID, target.Generation, time.Now().UTC()); err != nil {
		return &outbox.WaitError{Reason: "account_purge_finalization_pending", Cause: err, After: time.Minute}
	}
	return nil
}

func (w *PurgeWorker) guard(ctx context.Context, target *model.AccountPurge) error {
	now := time.Now().UTC()
	valid, err := w.Repository.AccountCleanupAdmissionValid(ctx, target.UserID, target.Generation, now)
	if err == nil && valid {
		return nil
	}
	reason := "admission_pending"
	if err == nil {
		reason, err = w.Repository.AccountCleanupAdmissionReason(ctx, target.UserID, target.Generation, now)
	}
	if reason == "administrator_protected" || reason == "account_superseded" {
		return &outbox.TerminalError{Reason: "account_purge_" + reason, Cause: err}
	}
	return &outbox.WaitError{Reason: "account_purge_" + reason, Cause: err, After: time.Minute}
}
