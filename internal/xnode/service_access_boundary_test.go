package xnode

import (
	"context"
	"testing"
	"time"
)

type serviceBoundaryRepository struct {
	Repository
	period Period
	reads  int
}

func (r *serviceBoundaryRepository) XNodePeriod(context.Context, string) (Period, error) {
	r.reads++
	return r.period, nil
}

func TestStandaloneAccessUsesExactHalfOpenTerm(t *testing.T) {
	start := time.Date(2030, 1, 2, 3, 4, 5, 123456789, time.UTC)
	access := ServiceAccess{StartedAt: start, ExpiresAt: start.Add(time.Hour), Strategy: "NO_RESET"}
	identity := Identity{ProxyUserID: 7, ServiceName: "standalone", CurrentPeriodID: "period"}
	for _, test := range []struct {
		name string
		at   time.Time
		open bool
	}{
		{"before start", start.Add(-time.Nanosecond), false},
		{"at start", start, true},
		{"before expiry", access.ExpiresAt.Add(-time.Nanosecond), true},
		{"at expiry", access.ExpiresAt, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &serviceBoundaryRepository{period: Period{ID: identity.CurrentPeriodID, ProxyUserID: identity.ProxyUserID, Status: "open"}}
			service := NewService(repo, nil, nil, nil, nil)
			service.now = func() time.Time { return test.at }
			period, err := service.servicePeriod(context.Background(), identity, &access)
			if err != nil || (period != nil) != test.open {
				t.Fatal("standalone term boundary changed", period, err)
			}
			if !test.open && repo.reads != 0 {
				t.Fatal("inactive term read accounting admission", repo.reads)
			}
		})
	}
}
