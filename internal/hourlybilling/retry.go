package hourlybilling

import (
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	queue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func deliveryWait(reason string, cause error, guards []database.HourlyDeliveryReady, skipped []string, now time.Time) error {
	return &queue.WaitError{Reason: reason, Cause: cause, After: deliveryRetryDelay(guards, skipped, now)}
}

// Another owner's paid boundary advances the global compile revision. A squad
// that was freshly ready may retry quickly with entirely new proofs. One second
// deliberately matches the established outbox clamp/admission cadence; an
// apparent subsecond delay here would not actually make dispatch subsecond.
// Missing/offline/expired proofs retain the ordinary five-second dependency wait.
func deliveryRetryDelay(guards []database.HourlyDeliveryReady, skipped []string, now time.Time) time.Duration {
	for _, id := range skipped {
		for _, guard := range guards {
			if guard.EnrollmentID == id && guard.CompileRevision > 0 && !guard.ObservedAt.IsZero() && !now.Before(guard.ObservedAt) && now.Sub(guard.ObservedAt) <= 4*time.Second && guard.ReadyUntil.After(now) {
				return time.Second
			}
		}
	}
	return 5 * time.Second
}
