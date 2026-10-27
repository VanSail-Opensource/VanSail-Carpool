package app

import (
	"context"
	"errors"
	"time"
)

// Each independent stage owns a deadline. Busy renewal/PAYG work cannot exhaust
// the feature/reconciliation budgets. Accounting has its existing independent lane.
func (a *Application) runPricingWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	cursors := [4]string{}
	stage := 0
	modelsRegistered := false
	lastCleanup := time.Time{}
	renewalStart := 0
	paygCursor := ""
	run := func(budget time.Duration, work func(context.Context) error) {
		bounded, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		if err := work(bounded); err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Debug("pricing stage deferred", "error", err)
		}
	}
	for {
		now := time.Now().UTC()
		run(time.Second, func(work context.Context) error {
			if err := a.observePricingHealth(work, now); err != nil {
				return err
			}
			return a.store.ObservePricingWorker(work, now)
		})
		// Renewal and PAYG preparation receive budgets independent of analytics.
		run(750*time.Millisecond, func(work context.Context) error {
			ids, err := a.store.DuePricingRenewals(work, now)
			if err != nil {
				return err
			}
			if len(ids) > 0 {
				renewalStart %= len(ids)
			}
			start := renewalStart
			for offset := range ids {
				id := ids[(start+offset)%len(ids)]
				renewalStart = (start + offset + 1) % len(ids)
				if _, err = a.store.PreparePricingRenewal(work, id, now); err != nil && !errors.Is(err, context.DeadlineExceeded) {
					a.logger.Debug("pricing renewal deferred", "purchase", id, "error", err)
				}
				if work.Err() != nil {
					return work.Err()
				}
			}
			return nil
		})
		run(750*time.Millisecond, func(work context.Context) error {
			storage, err := a.store.PricingStorage(work)
			if err != nil {
				return err
			}
			if storage["pressure"] != false {
				return nil
			}
			paygCursor, err = a.store.PricingPAYGBatchAfter(work, now, paygCursor)
			return err
		})
		current := stage % 4
		stage++
		run(1500*time.Millisecond, func(work context.Context) error {
			storage, err := a.store.PricingStorage(work)
			if err != nil {
				return err
			}
			if storage["pressure"] != false {
				return nil
			}
			if !modelsRegistered {
				if err = a.store.RegisterPricingModels(work, now); err != nil {
					return err
				}
				modelsRegistered = true
			}
			switch current {
			case 0:
				cursors[0], err = a.store.PricingFeatureBatch(work, now, cursors[0])
			case 1:
				cursors[1], err = a.store.PricingUserFeatureBatch(work, now, cursors[1])
			case 2:
				cursors[2], err = a.store.PricingOptimizationBatch(work, now, cursors[2])
			case 3:
				cursors[3], err = a.store.PricingRevenueForecastBatch(work, now, cursors[3])
			}
			return err
		})
		run(750*time.Millisecond, func(work context.Context) error {
			if err := a.store.ClosePricingOffers(work, now); err != nil {
				return err
			}
			return a.store.ReconcilePricingForecasts(work, now)
		})
		if now.Sub(lastCleanup) > time.Hour {
			run(time.Second, func(work context.Context) error {
				_, err := a.store.PricingCleanup(work, now)
				if err == nil {
					lastCleanup = now
				}
				return err
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
