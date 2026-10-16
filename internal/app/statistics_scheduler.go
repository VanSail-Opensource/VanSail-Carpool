package app

import (
	"context"
	"errors"
	"time"
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
}
