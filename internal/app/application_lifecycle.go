package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// Run starts provider workers before serving HTTP and waits for all internal
// workers before returning to the caller that owns Close.
func (a *Application) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("application run context is nil")
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	if err := a.upstreams.start(runCtx); err != nil {
		cancelRun()
		_ = a.upstreams.shutdown(context.Background())
		return fmt.Errorf("start upstream queues: %w", err)
	}
	profileCtx, cancelProfiles := context.WithTimeout(runCtx, 5*time.Minute)
	if a.usdt != nil {
		a.usdt.Start(runCtx)
	}
	if a.ipLookup != nil {
		a.ipLookup.StartBackground(runCtx, a.upstreams.iplookup["checks"])
	}
	if err := a.paymentProfiles.RefreshAll(profileCtx); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("BEPUSDT payment profile refresh was partial", "error", err)
	}
	cancelProfiles()
	if err := runCtx.Err(); err != nil {
		cancelRun()
		if a.ipLookup != nil {
			a.ipLookup.WaitBackground()
		}
		if a.usdt != nil {
			a.usdt.Wait()
		}
		_ = a.upstreams.shutdown(context.Background())
		return err
	}

	listenerDone := make(chan error, 1)
	go func() {
		a.logger.Info("VanSail Carpool listening", "address", a.httpServer.Addr)
		listenerDone <- a.httpServer.ListenAndServe()
	}()
	schedulerDone := make(chan struct{})
	schedulerStarted := make(chan struct{})
	go func() {
		defer close(schedulerDone)
		a.runScheduler(runCtx, schedulerStarted)
	}()
	<-schedulerStarted
	connectivityDone := make(chan error, 1)
	go func() { connectivityDone <- a.connectivity.Run(runCtx) }()
	statisticsDone := make(chan struct{})
	go func() {
		defer close(statisticsDone)
		a.runStatisticsScheduler(runCtx)
	}()
	abuseDone := make(chan struct{})
	go func() {
		defer close(abuseDone)
		a.runAbuseScheduler(runCtx)
	}()
	notificationsDone := make(chan struct{})
	go func() {
		defer close(notificationsDone)
		a.runNotificationScheduler(runCtx)
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case err := <-connectivityDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			runErr = fmt.Errorf("run connectivity checker: %w", err)
		}
		// Keep one completion available for the shutdown join.
		connectivityDone <- err
	case <-a.backups.RestartRequested():
		a.logger.Info("verified database restore staged; shutting down for pre-open swap")
	case err := <-listenerDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}
	shutdownErr := a.shutdownRuntime(cancelRun, schedulerDone, statisticsDone, abuseDone, notificationsDone)
	connectivityErr := <-connectivityDone
	if errors.Is(connectivityErr, context.Canceled) {
		connectivityErr = nil
	}
	return errors.Join(runErr, shutdownErr, connectivityErr)
}

func (a *Application) shutdownRuntime(cancelRun context.CancelFunc, schedulerDone, statisticsDone, abuseDone, notificationsDone <-chan struct{}) error {
	cancelRun()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
	serverErr := a.httpServer.Shutdown(shutdownCtx)
	cancel()
	if serverErr != nil {
		serverErr = errors.Join(serverErr, a.httpServer.Close())
	}

	// Database-dependent scheduler work and provider workers must be gone before
	// Run returns and the owner invokes Close on SQLite.
	<-schedulerDone
	<-statisticsDone
	<-abuseDone
	<-notificationsDone
	if a.usdt != nil {
		a.usdt.Wait()
	}
	if a.ipLookup != nil {
		a.ipLookup.WaitBackground()
	}
	flushCtx, cancelFlush := context.WithTimeout(context.Background(), 5*time.Second)
	flushErr := a.store.FlushGroupMessageFacts(flushCtx)
	cancelFlush()
	if flushErr != nil {
		flushErr = fmt.Errorf("flush group message facts: %w", flushErr)
	}
	queueErr := a.upstreams.shutdown(context.Background())
	return errors.Join(serverErr, flushErr, queueErr)
}

// Close checkpoints the authoritative on-disk database after runtime workers stop.
func (a *Application) Close() error {
	queueErr := a.upstreams.shutdown(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	checkpointErr := database.Checkpoint(ctx, a.store.DB(), true)
	return errors.Join(queueErr, checkpointErr, a.store.DB().Close())
}
