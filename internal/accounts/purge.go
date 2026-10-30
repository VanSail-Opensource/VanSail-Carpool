package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PurgeJob is handled by the existing durable outbox and upstream executors.
const PurgeJob = "account_purge"

// PurgeRepository retains accounting while removing current account material.
type PurgeRepository interface {
	AccountPurge(context.Context, string, int64) (*model.AccountPurge, error)
	CompleteAccountPurge(context.Context, string, int64, time.Time) error
}

// PurgeWorker closes native journals before completing local account cleanup.
type PurgeWorker struct {
	Repository PurgeRepository
	Native     interface {
		CloseAccountPeriods(context.Context, string) error
	}
	DeleteEmby func(context.Context, string) error
}

// HandleOutbox resumes only the generation selected by its durable job.
func (w *PurgeWorker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		UserID     string `json:"userId"`
		Generation int64  `json:"generation"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.UserID == "" || input.Generation < 1 {
		return errors.New("invalid account purge target")
	}
	target, err := w.Repository.AccountPurge(ctx, input.UserID, input.Generation)
	if err != nil || target == nil {
		return err
	}
	if target.Busy {
		return errors.New("account purge is waiting for earlier mutations")
	}
	if target.EmbyUncertain {
		return errors.New("account purge requires review of an uncertain Emby creation")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if target.RemoteEmbyID != "" {
		if err := w.DeleteEmby(bounded, target.RemoteEmbyID); err != nil {
			return err
		}
	}
	if err := w.Native.CloseAccountPeriods(bounded, target.UserID); err != nil {
		return err
	}
	return w.Repository.CompleteAccountPurge(bounded, target.UserID, target.Generation, time.Now().UTC())
}
