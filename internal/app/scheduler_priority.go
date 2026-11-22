package app

import (
	"context"
	"errors"
	"time"
)

// runOutboxAdmission keeps reserved and ordinary polling independent of both
// scheduled maintenance and the other lane's dispatch loop. Database claims
// retain the shared admission lock, bounded slots and durable ordering.
func (a *Application) runOutboxAdmission(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	urgentDone := make(chan struct{})
	go func() {
		defer close(urgentDone)
		a.runOutboxAdmissionLane(ctx, a.outbox.DrainPriority, "priority outbox dispatch pending", "XNODE_FIREWALL_ADMISSION_PENDING")
	}()
	defer func() { cancel(); <-urgentDone }()
	a.runOutboxAdmissionLane(ctx, a.outbox.Drain, "ordinary outbox dispatch pending", "OUTBOX_ADMISSION_PENDING")
}

func (a *Application) runOutboxAdmissionLane(ctx context.Context, drain func(context.Context, int) error, message, code string) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		call, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := drain(call, 32)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Warn(message, "error_code", code)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
