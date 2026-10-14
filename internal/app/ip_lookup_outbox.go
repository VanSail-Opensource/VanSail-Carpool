package app

import (
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

// registerIPLookupOutboxHandlers wires the two background repair lanes that
// refetch failed parts of partial lookups after the configured wait. Both
// handlers resolve their policy per execution, so administrators can stop or
// reshape ongoing chains by saving settings.
func registerIPLookupOutboxHandlers(worker *outbox.Worker, ipWorker *iplookup.Worker, live *iplookup.LiveService) error {
	if err := worker.Register(iplookup.ReportRetryOutboxKind, outbox.HandlerFunc(ipWorker.HandleReportRetryOutbox)); err != nil {
		return fmt.Errorf("register ip lookup report retry handler: %w", err)
	}
	if err := worker.Register(iplookup.LiveRetryOutboxKind, outbox.HandlerFunc(live.HandleLiveRetryOutbox)); err != nil {
		return fmt.Errorf("register ip lookup live retry handler: %w", err)
	}
	return nil
}
