package hourlybilling

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// The readiness proof must cover the bounded prefetch window, otherwise the
// database would always skip the next whole hour with ErrHourlyDeliveryPending.
func TestHourlyReadinessIncludesOnlyTheBoundedPrefetchWindow(t *testing.T) {
	at := time.Date(2026, 11, 10, 12, 30, 0, 0, time.UTC)
	native := &fixtureNative{readyHook: func(context.Context, string) (xnode.HourlyReadiness, error) {
		return xnode.HourlyReadiness{Ready: true, CompileRevision: 7, ObservedAt: at, ReadyUntil: at.Add(4 * time.Second)}, nil
	}}
	service := New(nil, nil, nil, nil)
	service.native = native
	service.now = func() time.Time { return at }
	expiring := at.Add(database.HourlyPrefetchWindow / 2)
	far := at.Add(database.HourlyPrefetchWindow + time.Second)
	overdue := at.Add(-time.Second)
	guards := service.deliveryGuards(context.Background(), []model.HourlyEnrollment{
		{ID: "expiring", OwnerID: "owner", GroupID: "expiring", Status: model.HourlyActive, PaidThrough: &expiring},
		{ID: "far", OwnerID: "owner", GroupID: "far", Status: model.HourlyActive, PaidThrough: &far},
		{ID: "overdue", OwnerID: "owner", GroupID: "overdue", Status: model.HourlyActive, PaidThrough: &overdue},
		{ID: "suspended", OwnerID: "owner", GroupID: "suspended", Status: model.HourlyFundingSuspended, PaidThrough: &overdue},
	})
	got := map[string]bool{}
	for _, guard := range guards {
		got[guard.EnrollmentID] = true
	}
	if !got["expiring"] || !got["overdue"] || got["far"] || got["suspended"] {
		t.Fatalf("readiness scope = %+v", got)
	}
}
