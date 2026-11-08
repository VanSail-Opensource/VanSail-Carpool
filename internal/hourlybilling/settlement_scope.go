package hourlybilling

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// Skipped rows acquired no new paid authority. The agent's hard paid boundary
// already denies them; syncing them would only delay freshly committed work and
// could age every valid contention proof while waiting for an offline peer.
func settlementApplications(result database.HourlyOwnerSettlementResult) []model.HourlyEnrollment {
	skipped := make(map[string]bool, len(result.SkippedEnrollmentIDs))
	for _, id := range result.SkippedEnrollmentIDs {
		skipped[id] = true
	}
	apply := make([]model.HourlyEnrollment, 0, len(result.Enrollments))
	for _, e := range result.Enrollments {
		if !skipped[e.ID] {
			apply = append(apply, e)
		}
	}
	return apply
}
