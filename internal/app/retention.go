package app

import (
	"context"
	"time"
)

func (a *Application) prunePAYGSamples(ctx context.Context, now time.Time) {
	counts, err := a.store.PrunePAYGTransientRecords(ctx, now)
	if err != nil {
		a.logger.Error("PAYG sample retention failed", "error", err, "deleted", counts)
	} else if counts["xnode_payg_minutes"]+counts["xnode_payg_windows"] > 0 {
		a.logger.Info("PAYG samples expired", "deleted", counts)
	}
}
