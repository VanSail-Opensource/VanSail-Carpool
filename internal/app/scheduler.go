package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/maintenance"
)

func (a *Application) runScheduler(ctx context.Context, startupComplete chan<- struct{}) {
	outboxTicker := time.NewTicker(5 * time.Second)
	groupFactsTicker := time.NewTicker(time.Second)
	transitionTicker := time.NewTicker(30 * time.Second)
	maintenanceTicker := time.NewTicker(10 * time.Minute)
	starsTicker := time.NewTicker(5 * time.Minute)
	maintenanceTimer := time.NewTimer(time.Until(a.nextMaintenance(time.Now())))
	defer outboxTicker.Stop()
	defer groupFactsTicker.Stop()
	defer transitionTicker.Stop()
	defer maintenanceTicker.Stop()
	defer starsTicker.Stop()
	defer maintenanceTimer.Stop()
	a.configureTelegram(ctx)
	startupNow := time.Now().UTC()
	if err := a.store.RecoverOutbox(ctx, startupNow, startupNow); err != nil {
		a.logger.Error("startup outbox recovery failed", "error", err)
	}
	priorityCtx, stopPriority := context.WithCancel(ctx)
	priorityDone := make(chan struct{})
	go func() { defer close(priorityDone); a.runPriorityOutbox(priorityCtx) }()
	defer func() { stopPriority(); <-priorityDone }()
	if err := a.store.EnqueueUnlinkedPaidUsers(ctx, startupNow); err != nil {
		a.logger.Error("startup unlinked paid user scan failed", "error", err)
	}
	if err := a.store.EnqueueContinuityBacklog(ctx, startupNow); err != nil {
		a.logger.Error("startup entitlement continuity scan failed", "error", err)
	}
	if err := a.store.EnqueueDueEntitlementTransitions(ctx, startupNow); err != nil {
		a.logger.Error("startup entitlement transition scan failed", "error", err)
	}
	if err := a.outbox.Drain(ctx, 50); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("startup outbox drain failed", "error", err)
	}
	if err := a.store.EnqueueDueEntitlementTransitions(ctx, startupNow); err != nil {
		a.logger.Error("startup successor transition scan failed", "error", err)
	}
	if err := a.catalog.ProcessDueAutoRenewals(ctx, startupNow); err != nil {
		a.logger.Error("startup automatic renewal scan failed", "error", err)
	}
	close(startupComplete)
	for {
		select {
		case <-ctx.Done():
			return
		case <-outboxTicker.C:
			if err := a.outbox.Drain(ctx, 20); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error("outbox drain failed", "error", err)
			}
		case <-groupFactsTicker.C:
			if err := a.store.FlushGroupMessageFacts(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error("group message fact flush failed", "error", err)
			}
		case now := <-transitionTicker.C:
			if err := a.store.RecoverOutbox(ctx, now.UTC().Add(-2*time.Minute), now.UTC()); err != nil {
				a.logger.Error("outbox lease recovery failed", "error", err)
			}
			if err := a.store.EnqueueDueEntitlementTransitions(ctx, now.UTC()); err != nil {
				a.logger.Error("entitlement transition scan failed", "error", err)
			}
			if err := a.outbox.Drain(ctx, 50); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error("rollover settlement drain failed", "error", err)
			}
			if err := a.store.EnqueueDueEntitlementTransitions(ctx, now.UTC()); err != nil {
				a.logger.Error("successor transition scan failed", "error", err)
			}
			if err := a.catalog.ProcessDueAutoRenewals(ctx, now.UTC()); err != nil {
				a.logger.Error("automatic renewal scan failed", "error", err)
			}
			if err := a.store.ExpireStalePaymentOrders(ctx, now.UTC()); err != nil {
				a.logger.Error("payment expiry scan failed", "error", err)
			}
		case <-maintenanceTicker.C:
			a.configureTelegram(ctx)
		case <-starsTicker.C:
			a.reconcileStars(ctx)
		case <-maintenanceTimer.C:
			maintenanceCtx, cancel := context.WithTimeout(ctx, maintenance.RunTimeout)
			maintenanceNow := time.Now().UTC()
			maintenanceResult, err := runMaintenanceTask(maintenanceCtx, a.maintenance, a.config.DatabasePath, maintenanceNow, false)
			if err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error("daily maintenance failed", "error", err)
			} else if err == nil {
				if maintenanceResult.BackupRetentionWarning != nil {
					a.logger.Warn("verified backup retention failed after daily cleanup", "error", maintenanceResult.BackupRetentionWarning)
				}
				if maintenanceResult.MigrationSnapshotRetentionWarning != nil {
					a.logger.Error("migration snapshot retention failed", "error", maintenanceResult.MigrationSnapshotRetentionWarning)
				}
			}
			cancel()
			maintenanceTimer.Reset(time.Until(a.nextMaintenance(time.Now())))
		}
	}
}

func (a *Application) nextMaintenance(now time.Time) time.Time {
	local := now.In(a.config.Timezone)
	next := time.Date(local.Year(), local.Month(), local.Day(), a.config.BackupHour, 0, 0, 0, a.config.Timezone)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
