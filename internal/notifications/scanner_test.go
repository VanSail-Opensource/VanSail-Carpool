package notifications

import (
	"context"
	"math"
	"testing"
	"time"
)

type scanRepositoryStub struct{ traffic []TrafficUser }

func (r *scanRepositoryStub) EnqueueExpiryReminderNotifications(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (r *scanRepositoryStub) EnqueueTrafficThresholdNotification(_ context.Context, user TrafficUser, _ time.Time) (bool, error) {
	r.traffic = append(r.traffic, user)
	return user.ID > 0, nil
}

type trafficRemoteStub struct{ users []TrafficUser }

func (r trafficRemoteStub) ListNotificationUsers(context.Context, string, int) ([]TrafficUser, *string, bool, error) {
	return r.users, nil, false, nil
}

type hourlyFundingScanStub struct {
	scanRepositoryStub
	warnings int
}

func (r *hourlyFundingScanStub) EnqueueHourlyFundingWarnings(context.Context, time.Time) (int, error) {
	r.warnings++
	return 1, nil
}

func TestScannerEnqueuesHourlyFundingWarningsWhenSupported(t *testing.T) {
	t.Parallel()
	repository := &hourlyFundingScanStub{}
	if err := NewScanner(repository, trafficRemoteStub{}, nil).Scan(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("Scan(): %v", err)
	}
	if repository.warnings != 1 {
		t.Fatalf("hourly funding scan calls = %d", repository.warnings)
	}
}

func TestScannerUsesStrictOverflowSafeTrafficBoundary(t *testing.T) {
	t.Parallel()
	if aboveNinetyPercent(900, 1000) {
		t.Fatal("exactly 90% was considered above 90%")
	}
	if !aboveNinetyPercent(901, 1000) || !aboveNinetyPercent(math.MaxInt64, math.MaxInt64-1) {
		t.Fatal("aboveNinetyPercent rejected an eligible value")
	}
	repository := &scanRepositoryStub{}
	reset := time.Now().UTC()
	scanner := NewScanner(repository, trafficRemoteStub{users: []TrafficUser{
		{ID: 1, UsedBytes: 900, LimitBytes: 1000},
		{ID: 2, OwnerID: "native-owner", AccountGeneration: 7, PeriodID: "current-paid-period", UsedBytes: 901, LimitBytes: 1000, ResetStrategy: "NO_RESET", LastTrafficResetAt: &reset},
	}}, nil)
	if err := scanner.Scan(context.Background(), reset); err != nil {
		t.Fatalf("Scan(): %v", err)
	}
	if len(repository.traffic) != 1 || repository.traffic[0] != scanner.remote.(trafficRemoteStub).users[1] {
		t.Fatalf("traffic calls = %+v", repository.traffic)
	}
}
