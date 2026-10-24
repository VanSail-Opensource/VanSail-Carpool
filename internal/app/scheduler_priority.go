package app

import (
	"context"
	"errors"
	"time"
)

// runOutboxAdmission keeps both reserved and ordinary dispatch moving while
// scheduled maintenance or entitlement scans wait. Both use the same executor.
func (a *Application) runOutboxAdmission(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		call, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := a.outbox.DrainPriority(call, 32)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Warn("priority outbox dispatch pending", "error_code", "XNODE_FIREWALL_ADMISSION_PENDING")
		}
		call, cancel = context.WithTimeout(ctx, 5*time.Second)
		err = a.outbox.Drain(call, 32)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Warn("ordinary outbox dispatch pending", "error_code", "OUTBOX_ADMISSION_PENDING")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
