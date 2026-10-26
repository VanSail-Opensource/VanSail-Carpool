package app

import (
	"context"
	"errors"
	"time"
)

// A single discretionary worker is joined on shutdown. Accounting and native
// traffic acknowledgment never wait for it or share its execution budget.
func (a *Application) runPricingWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	stage := 0
	nodeCursor, userCursor, productCursor := "", "", ""
	lastCleanup := time.Time{}
	revenueCursor := ""
	modelsRegistered := false
	for {
		now := time.Now().UTC()
		work, cancel := context.WithTimeout(ctx, 3*time.Second)
		if err := a.observePricingHealth(work, now); err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Warn("pricing health unavailable", "error", err)
		} else if err == nil {
			if err = a.store.ObservePricingWorker(work, now); err != nil {
				a.logger.Warn("pricing observation watermark unavailable", "error", err)
			}
		}
		ids, err := a.store.DuePricingRenewals(work, now)
		if err == nil {
			for _, id := range ids {
				if _, err = a.store.PreparePricingRenewal(work, id, now); err != nil {
					a.logger.Warn("pricing renewal preparation deferred", "purchase", id, "error", err)
				}
				if work.Err() != nil {
					break
				}
			}
		}
		if work.Err() == nil {
			storage, e := a.store.PricingStorage(work)
			if e == nil && storage["pressure"] == false {
				if e = a.store.PricingPAYGBatch(work, now); e != nil {
					a.logger.Debug("PAYG calculation deferred", "error", e)
				}
				if !modelsRegistered {
					e = a.store.RegisterPricingModels(work, now)
					modelsRegistered = e == nil
				}
				if e == nil {
					switch stage % 4 {
					case 0:
						nodeCursor, e = a.store.PricingFeatureBatch(work, now, nodeCursor)
					case 1:
						userCursor, e = a.store.PricingUserFeatureBatch(work, now, userCursor)
					case 2:
						productCursor, e = a.store.PricingOptimizationBatch(work, now, productCursor)
					case 3:
						revenueCursor, e = a.store.PricingRevenueForecastBatch(work, now, revenueCursor)
					}
					stage++
				}
				if e != nil {
					a.logger.Debug("pricing analytics deferred", "error", e)
				}
			}
			if e = a.store.ClosePricingOffers(work, now); e == nil {
				e = a.store.ReconcilePricingForecasts(work, now)
			}
			if e != nil {
				a.logger.Debug("pricing reconciliation deferred", "error", e)
			}
		}
		if now.Sub(lastCleanup) > time.Hour && work.Err() == nil {
			if _, e := a.store.PricingCleanup(work, now); e == nil {
				lastCleanup = now
			} else {
				a.logger.Debug("pricing cleanup deferred", "error", e)
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
