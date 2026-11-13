package catalog

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type dashboardReadHook struct {
	*catalogProxy
	read func(model.User) (RemoteDashboard, error)
}

func (r *dashboardReadHook) DashboardForMember(_ context.Context, user model.User) (RemoteDashboard, error) {
	return r.read(user)
}

func TestDashboardInvalidationRetiresInFlightCredentialsAndFallback(t *testing.T) {
	t.Parallel()
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint("provider_failure_", failure), func(t *testing.T) {
			now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
			user := model.User{ID: "owner", AccountGeneration: 7}
			remote := &dashboardReadHook{catalogProxy: &catalogProxy{}}
			service := NewService(&catalogRepository{}, remote, time.Minute)
			service.now = func() time.Time { return now }
			old := RemoteDashboard{SubscriptionURL: "old-credential"}
			service.cacheDashboard(user, old, now.Add(-2*time.Minute), service.cacheEpoch)
			remote.read = func(model.User) (RemoteDashboard, error) {
				// A credential rotation and a newer read finish while this read waits.
				service.InvalidateDashboard(user.ID)
				service.cacheDashboard(user, RemoteDashboard{SubscriptionURL: "new-credential"}, now, service.cacheEpoch)
				if failure {
					return RemoteDashboard{}, errors.New("provider unavailable")
				}
				return old, nil
			}
			got, err := service.Dashboard(context.Background(), user)
			if err != nil || got.SubscriptionURL != nil || got.SubscriptionReady || !got.StatisticsStale {
				t.Fatalf("retired result escaped: %+v, %v", got, err)
			}
			if cached := service.cache[user.ID]; cached.value.SubscriptionURL != "new-credential" {
				t.Fatalf("retired read replaced current cache: %+v", cached)
			}
			got, err = service.Dashboard(context.Background(), user)
			if err != nil || got.SubscriptionURL == nil || *got.SubscriptionURL != "new-credential" {
				t.Fatalf("current read did not retain rotated credential: %+v, %v", got, err)
			}
		})
	}
}

func TestDashboardCacheDoesNotCrossAccountGeneration(t *testing.T) {
	t.Parallel()
	remote := &dashboardReadHook{catalogProxy: &catalogProxy{}}
	calls := 0
	remote.read = func(user model.User) (RemoteDashboard, error) {
		calls++
		return RemoteDashboard{SubscriptionURL: fmt.Sprintf("generation-%d", user.AccountGeneration)}, nil
	}
	service := NewService(&catalogRepository{}, remote, time.Hour)
	for _, generation := range []int64{7, 8, 8} {
		got, err := service.Dashboard(context.Background(), model.User{ID: "owner", AccountGeneration: generation})
		if err != nil || got.SubscriptionURL == nil || *got.SubscriptionURL != fmt.Sprintf("generation-%d", generation) {
			t.Fatalf("generation %d: %+v, %v", generation, got, err)
		}
	}
	if calls != 2 {
		t.Fatalf("provider calls = %d, want one per generation", calls)
	}
}

func TestDashboardCacheDoesNotCrossLegacyIdentity(t *testing.T) {
	t.Parallel()
	remote := &catalogProxy{dashboard: RemoteDashboard{SubscriptionURL: "first-credential"}}
	service := NewService(&catalogRepository{}, remote, time.Hour)
	identity := "first"
	user := model.User{ID: "owner", ProxyUserID: &identity}
	if _, err := service.Dashboard(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	identity = "second"
	remote.dashboard.SubscriptionURL = "second-credential"
	got, err := service.Dashboard(context.Background(), user)
	if err != nil || got.SubscriptionURL == nil || *got.SubscriptionURL != "second-credential" || remote.dashboardCalls != 2 {
		t.Fatalf("legacy identity reused another credential: %+v, %v", got, err)
	}
}
