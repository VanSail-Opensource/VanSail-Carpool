package iplookup

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// executeBackground starts lease ownership and attempt timing only after admission.
func (w *Worker) executeBackground(ctx context.Context, repo backgroundRepository, id string) error {
	// An owner identifies this execution, not this process. Reacquisition after
	// expiry must never authorize a paused earlier execution from the same worker.
	owner, err := ids.New()
	if err != nil {
		return err
	}
	claimed, err := repo.ClaimIPLookupLease(ctx, id, owner, w.now().UTC())
	if err != nil || !claimed {
		return err
	}
	runCtx, stopRun := context.WithCancel(WithRunLease(ctx, owner))
	defer stopRun()
	heartbeatCtx, cancel := context.WithCancel(runCtx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			if err := repo.HeartbeatIPLookup(heartbeatCtx, id, owner, w.now().UTC()); err != nil {
				stopRun()
				return
			}
			select {
			case <-heartbeatCtx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	defer func() { cancel(); <-done }()
	op, err := repo.ProviderOperationByID(runCtx, id)
	if err != nil {
		return err
	}
	return w.executeOperation(runCtx, op)
}
