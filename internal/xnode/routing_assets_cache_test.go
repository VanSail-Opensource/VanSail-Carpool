package xnode

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func routingAssetCacheConfig() GlobalConfig {
	config := DefaultGlobal()
	config.GeoSiteURL = "https://assets.example/geosite.dat"
	hours := 1
	config.GeodataIntervalHours = &hours
	return config
}

func routingAssetFixture(hash string, selectors []string) routingAssetResult {
	domains := map[string][]string{}
	for _, selector := range selectors {
		domains[selector] = []string{"domain:example.com"}
	}
	return routingAssetResult{hash: hash, domains: domains}
}

func TestRoutingAssetsRetainAlternatingMemberSelections(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	calls, publications := 0, 0
	assets := &routingAssets{publish: func() { publications++ }}
	load := func(_ context.Context, _ GlobalConfig, selectors []string) (routingAssetResult, error) {
		calls++
		return routingAssetFixture("same-file", selectors), nil
	}
	// Repeated passes across many member plans must reuse each selected
	// projection rather than replacing one shared global cache slot.
	for range 3 {
		for member := range 23 {
			selectors := []string{fmt.Sprintf("geosite:category-%d", member)}
			data, err := assets.resolve(context.Background(), routingAssetCacheConfig(), selectors, clock, load)
			if err != nil || len(data.domains[selectors[0]]) != 1 {
				t.Fatal("selector projection lost", data, err)
			}
		}
	}
	if calls != 23 || publications != 1 {
		t.Fatalf("downloads=%d publications=%d; unchanged source was rebuilt", calls, publications)
	}
}

func TestRoutingAssetsContentChangeInvalidatesOtherSelections(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	hash, calls, publications := "first", 0, 0
	assets := &routingAssets{publish: func() { publications++ }}
	load := func(_ context.Context, _ GlobalConfig, selectors []string) (routingAssetResult, error) {
		calls++
		return routingAssetFixture(hash, selectors), nil
	}
	read := func(selector string) routingAssetResult {
		t.Helper()
		data, err := assets.resolve(context.Background(), routingAssetCacheConfig(), []string{selector}, clock, load)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	read("geosite:google")
	now = now.Add(30 * time.Minute)
	read("geosite:youtube")
	now = now.Add(30 * time.Minute)
	hash = "second"
	read("geosite:google")
	// YouTube's own deadline is still 30 minutes away, but another selection
	// has proved that this source's bytes changed; the old generation is invalid.
	if data := read("geosite:youtube"); data.hash != "second" {
		t.Fatal("mixed source generations", data.hash)
	}
	read("geosite:google")
	if calls != 4 || publications != 2 {
		t.Fatalf("downloads=%d publications=%d", calls, publications)
	}
}

func TestRoutingAssetsConcurrentRefreshDownloadsOnce(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	var calls atomic.Int64
	entered, release := make(chan struct{}), make(chan struct{})
	assets := &routingAssets{}
	load := func(_ context.Context, _ GlobalConfig, selectors []string) (routingAssetResult, error) {
		if calls.Add(1) == 2 {
			close(entered)
			<-release
		}
		return routingAssetFixture("verified", selectors), nil
	}
	config, selectors := routingAssetCacheConfig(), []string{"geosite:google"}
	if _, err := assets.resolve(context.Background(), config, selectors, clock, load); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	var workers sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := assets.resolve(context.Background(), config, selectors, clock, load)
			errors <- err
		}()
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("refresh did not enter loader")
	}
	close(release)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("concurrent refreshes repeated download", calls.Load())
	}
}
