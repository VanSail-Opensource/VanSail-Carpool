package iplookup

import (
	"context"
	"time"
)

// executeBackground starts lease ownership and attempt timing only after admission.
func (w *Worker) executeBackground(ctx context.Context, repo backgroundRepository, id string) error {
	b := w.background
	claimed, err := repo.ClaimIPLookupLease(ctx, id, b.owner, w.now().UTC())
	if err != nil || !claimed {
		return err
	}
	runCtx, stopRun := context.WithCancel(ctx)
	defer stopRun()
	heartbeatCtx, cancel := context.WithCancel(runCtx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			if err := repo.HeartbeatIPLookup(heartbeatCtx, id, b.owner, w.now().UTC()); err != nil {
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
