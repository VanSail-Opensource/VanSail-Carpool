package app

import "context"

func (a *Application) startDatabaseGuard(parent context.Context, cancelRun context.CancelFunc) func() {
	ctx, stop := context.WithCancel(parent)
	done := make(chan struct{})
	observationsDone := make(chan struct{})
	go func() { defer close(observationsDone); a.observeDatabase(ctx) }()
	go func() {
		defer close(done)
		if a.databaseOwner == nil {
			return
		}
		if err := a.databaseOwner.Monitor(ctx); err != nil {
			a.logger.Error("database ownership lost; stopping application", "error", err)
			cancelRun()
		}
	}()
	return func() { stop(); <-done; <-observationsDone }
}
