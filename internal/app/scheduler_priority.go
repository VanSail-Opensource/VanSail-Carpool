package app

import (
	"context"
	"errors"
	"time"
)

// runPriorityOutbox reserves admission on the existing dispatcher so a slow
// provider operation or daily maintenance cannot delay a connected node's bans.
func (a *Application) runPriorityOutbox(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		call, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := a.outbox.DrainPriority(call, 32)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Warn("priority outbox dispatch pending", "error_code", "XNODE_FIREWALL_ADMISSION_PENDING")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
