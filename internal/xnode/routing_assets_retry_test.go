package xnode

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRoutingAssetsColdFailureBackoffAndCancellation(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	assets, calls := &routingAssets{}, 0
	config, selectors := routingAssetCacheConfig(), []string{"geosite:missing"}
	load := func(context.Context, GlobalConfig, []string) (routingAssetResult, error) {
		calls++
		return routingAssetResult{}, ErrInvalid
	}
	for _, step := range []struct {
		advance time.Duration
		calls   int
	}{{0, 1}, {0, 1}, {time.Minute, 2}, {4 * time.Minute, 2}, {time.Minute, 3}, {14 * time.Minute, 3}, {time.Minute, 4}} {
		now = now.Add(step.advance)
		if data, err := assets.resolve(context.Background(), config, selectors, clock, load); !errors.Is(err, ErrInvalid) || data.hash != "" || calls != step.calls {
			t.Fatal("cold failure lost backoff or leaked unrelated data", data, err, calls, step.calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	other := []string{"geosite:cancelled"}
	_, err := assets.resolve(ctx, config, other, clock, func(context.Context, GlobalConfig, []string) (routingAssetResult, error) {
		cancel()
		return routingAssetResult{}, context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = assets.resolve(context.Background(), config, other, clock, func(_ context.Context, _ GlobalConfig, selected []string) (routingAssetResult, error) {
		return routingAssetFixture("valid", selected), nil
	}); err != nil {
		t.Fatal("one caller's cancellation poisoned the cache", err)
	}
}

func TestRoutingAssetsStaleFallbackStaysWithinExactConfiguration(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	assets := &routingAssets{}
	config, selectors := routingAssetCacheConfig(), []string{"geosite:google"}
	load := func(_ context.Context, _ GlobalConfig, selected []string) (routingAssetResult, error) {
		return routingAssetFixture("old-verified", selected), nil
	}
	if _, err := assets.resolve(context.Background(), config, selectors, clock, load); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	calls := 0
	failed := func(context.Context, GlobalConfig, []string) (routingAssetResult, error) {
		calls++
		return routingAssetResult{}, ErrUnavailable
	}
	for range 2 {
		if data, err := assets.resolve(context.Background(), config, selectors, clock, failed); err != nil || data.hash != "old-verified" {
			t.Fatal("same-key verified fallback disappeared", data, err)
		}
	}
	if calls != 1 {
		t.Fatal("failed refresh was not deferred", calls)
	}
	changed := []GlobalConfig{config, config, config}
	changed[0].GeoSiteURL = "https://new.example/geosite.dat"
	changed[1].GeoSiteHash = "new-pin"
	hours := 2
	changed[2].GeodataIntervalHours = &hours
	for _, next := range changed {
		if data, err := assets.resolve(context.Background(), next, selectors, clock, failed); !errors.Is(err, ErrUnavailable) || data.hash != "" {
			t.Fatal("different source/pin/schedule reused stale data", data, err)
		}
	}
}

func TestRoutingAssetsRefreshDeadlineIgnoresExpiredUnrelatedEntry(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	assets := &routingAssets{}
	service := &Service{routingAssets: assets, now: clock}
	config := routingAssetCacheConfig()
	load := func(_ context.Context, _ GlobalConfig, selected []string) (routingAssetResult, error) {
		return routingAssetFixture("same", selected), nil
	}
	if _, err := assets.resolve(context.Background(), config, []string{"geosite:unused"}, clock, load); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Minute)
	if _, err := assets.resolve(context.Background(), config, []string{"geosite:used"}, clock, load); err != nil {
		t.Fatal(err)
	}
	want := now.Add(time.Hour)
	now = now.Add(30 * time.Minute)
	if next := service.routingRefreshAt(); !next.Equal(want) {
		t.Fatal("expired unused selection suppressed a future refresh", next, want)
	}
	if _, err := assets.resolve(context.Background(), config, []string{"geosite:unused"}, clock, func(context.Context, GlobalConfig, []string) (routingAssetResult, error) {
		return routingAssetResult{}, ErrUnavailable
	}); err != nil {
		t.Fatal("verified stale refresh failed", err)
	}
	if next := service.routingRefreshAt(); !next.Equal(now.Add(time.Minute)) {
		t.Fatal("stale retry deadline was omitted", next)
	}
}
