package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
)

const statisticsRefreshTimeout = 5 * time.Minute

func nextStatisticsRefresh(now time.Time) time.Time {
	return now.Truncate(30 * time.Minute).Add(30 * time.Minute)
}

func (a *Application) runStatisticsScheduler(ctx context.Context) {
	a.refreshProductStatistics(ctx, time.Now().UTC())
	timer := time.NewTimer(time.Until(nextStatisticsRefresh(time.Now())))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-timer.C:
			a.refreshProductStatistics(ctx, now.UTC())
			timer.Reset(time.Until(nextStatisticsRefresh(time.Now())))
		}
	}
}

func (a *Application) refreshProductStatistics(ctx context.Context, now time.Time) {
	compensationCtx, cancelCompensation := context.WithTimeout(ctx, statisticsRefreshTimeout)
	if err := a.compensation.Observe(compensationCtx, now); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("node compensation observation skipped", "error", err)
	}
	cancelCompensation()
	refreshCtx, cancelRefresh := context.WithTimeout(ctx, statisticsRefreshTimeout)
	if err := a.statistics.Refresh(refreshCtx, now); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("statistics partition refresh was partial", "error", err)
	}
	cancelRefresh()
	if err := a.statistics.RefreshGeochecks(ctx, now); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("node geocheck refresh scheduling failed", "error", err)
	}
	hostCtx, cancelHosts := context.WithTimeout(ctx, statisticsRefreshTimeout)
	defer cancelHosts()
	actorID, err := a.statisticsActorID(hostCtx)
	if errors.Is(err, database.ErrNotFound) {
		a.logger.Warn("host multiplier update skipped until an administrator account exists")
		return
	}
	if err != nil {
		a.logger.Error("host multiplier actor lookup failed", "error", err)
		return
	}
	if err := productstats.QueueHostMultiplierUpdates(hostCtx, newRemnaAdapter(a.settings, a.upstreams.remnawave), a.store, actorID, now); err != nil {
		a.logger.Error("host multiplier operation scheduling was partial", "error", err)
	}
}

func (a *Application) statisticsActorID(ctx context.Context) (string, error) {
	for _, telegramID := range a.config.AdminTelegramIDs {
		user, err := a.store.UserByTelegramID(ctx, telegramID)
		if err == nil {
			return user.ID, nil
		}
		if !errors.Is(err, database.ErrNotFound) {
			return "", err
		}
	}
	return "", database.ErrNotFound
}
