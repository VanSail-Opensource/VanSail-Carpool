package hourlybilling

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestFreshContentionRetriesAtRealQueueFloorWhileOfflineProofsWait(t *testing.T) {
	at := time.Now().UTC()
	guard := database.HourlyDeliveryReady{EnrollmentID: "ready", ObservedAt: at.Add(-time.Second), ReadyUntil: at.Add(time.Second), CompileRevision: 7}
	for _, test := range []struct {
		name    string
		guards  []database.HourlyDeliveryReady
		skipped []string
		at      time.Time
		want    time.Duration
	}{
		{"fresh revision contention", []database.HourlyDeliveryReady{guard}, []string{"ready"}, at, time.Second},
		{"offline missing proof", nil, []string{"offline"}, at, 5 * time.Second},
		{"expired evidence", []database.HourlyDeliveryReady{guard}, []string{"ready"}, at.Add(5 * time.Second), 5 * time.Second},
		{"another healthy squad", []database.HourlyDeliveryReady{guard}, []string{"offline"}, at, 5 * time.Second},
		{"no rejected evidence", []database.HourlyDeliveryReady{guard}, nil, at, 5 * time.Second},
		{"clock before proof", []database.HourlyDeliveryReady{guard}, []string{"ready"}, at.Add(-2 * time.Second), 5 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := deliveryRetryDelay(test.guards, test.skipped, test.at); got != test.want {
				t.Fatalf("retry delay = %s, want %s", got, test.want)
			}
		})
	}
}

func TestSkippedUnpaidEnrollmentNeverEntersNativeApplication(t *testing.T) {
	result := database.HourlyOwnerSettlementResult{Enrollments: []model.HourlyEnrollment{
		{ID: "skipped", Status: model.HourlyActive}, {ID: "paid", Status: model.HourlyActive}, {ID: "suspended", Status: model.HourlyFundingSuspended},
	}, SkippedEnrollmentIDs: []string{"skipped"}}
	apply := settlementApplications(result)
	if len(apply) != 2 || apply[0].ID != "paid" || apply[1].ID != "suspended" {
		t.Fatal("skipped unpaid authority entered native work or committed changes were lost")
	}
}
