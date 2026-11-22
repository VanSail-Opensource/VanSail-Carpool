package xnode

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRoutingAssetsBoundRetainedMemoryAndKeys(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	config := routingAssetCacheConfig()
	assets, calls := &routingAssets{}, 0
	load := func(_ context.Context, _ GlobalConfig, selected []string) (routingAssetResult, error) {
		calls++
		return routingAssetFixture("same", selected), nil
	}
	for index := range maxRoutingAssetEntries + 1 {
		config.GeoSiteURL = fmt.Sprintf("https://assets-%d.example/geosite.dat", index)
		if _, err := assets.resolve(context.Background(), config, []string{"geosite:google"}, clock, load); err != nil {
			t.Fatal(err)
		}
	}
	if len(assets.entries) != maxRoutingAssetEntries || len(assets.sources) != maxRoutingAssetEntries {
		t.Fatal("cache key/source metadata exceeded bound", len(assets.entries), len(assets.sources))
	}
	config.GeoSiteURL = "https://assets-0.example/geosite.dat"
	if _, err := assets.resolve(context.Background(), config, []string{"geosite:google"}, clock, load); err != nil || calls != maxRoutingAssetEntries+2 {
		t.Fatal("oldest selection did not evict", calls, err)
	}
	// Each synthetic validated projection fits the parser's 8MiB text limit,
	// but their combined retention must remain bounded independently of count.
	assets, calls = &routingAssets{}, 0
	large := strings.Repeat("x", 6<<20)
	load = func(_ context.Context, _ GlobalConfig, selected []string) (routingAssetResult, error) {
		calls++
		return routingAssetResult{hash: "same", domains: map[string][]string{selected[0]: {large}}}, nil
	}
	for _, selector := range []string{"geosite:first", "geosite:second", "geosite:first", "geosite:third"} {
		if _, err := assets.resolve(context.Background(), config, []string{selector}, clock, load); err != nil {
			t.Fatal(err)
		}
	}
	if assets.bytes > maxRoutingAssetCacheBytes || len(assets.entries) != 2 || calls != 3 {
		t.Fatal("retained byte bound failed", assets.bytes, len(assets.entries), calls)
	}
	if _, err := assets.resolve(context.Background(), config, []string{"geosite:first"}, clock, load); err != nil || calls != 3 {
		t.Fatal("recently used projection evicted before older projection", err, calls)
	}
	if _, err := assets.resolve(context.Background(), config, []string{"geosite:second"}, clock, load); err != nil || calls != 4 {
		t.Fatal("older projection did not evict under byte pressure", err, calls)
	}
}

func TestRoutingAssetsShareLargeSelectorsAcrossCombinations(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	assets, calls := &routingAssets{}, 0
	load := func(_ context.Context, _ GlobalConfig, selected []string) (routingAssetResult, error) {
		calls++
		data := routingAssetFixture("same", selected)
		// Separate downloads allocate independent strings. Their verified
		// common selector must retain only one copy under the same file hash.
		data.domains["geosite:common"] = []string{strings.Repeat("x", 2<<20)}
		return data, nil
	}
	for range 2 {
		for member := range 23 {
			selected := []string{"geosite:common", fmt.Sprintf("geosite:category-%d", member)}
			if _, err := assets.resolve(context.Background(), routingAssetCacheConfig(), selected, clock, load); err != nil {
				t.Fatal(err)
			}
		}
	}
	if calls != 23 || len(assets.entries) != 23 || len(assets.domains) != 24 || assets.bytes > 4<<20 {
		t.Fatal("common selectors duplicated or thrashed the byte limit", calls, len(assets.entries), len(assets.domains), assets.bytes)
	}
}

func TestRoutingAssetsOversizedProjectionKeepsRefreshFence(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	assets, calls, publications := &routingAssets{}, 0, 0
	assets.publish = func() { publications++ }
	service := &Service{routingAssets: assets, now: clock}
	domains := map[string][]string{}
	selectors := make([]string, 32768)
	for index := range selectors {
		selectors[index] = fmt.Sprintf("geosite:%0120d", index)
		domains[selectors[index]] = []string{"domain:example.com"}
	}
	// Selector-map overhead can exceed the retention budget even though this
	// shape is below the parser's 100k-expanded-domain and 8MiB-text limits.
	load := func(context.Context, GlobalConfig, []string) (routingAssetResult, error) {
		calls++
		return routingAssetResult{hash: "same", domains: domains}, nil
	}
	for range 2 {
		data, err := assets.resolve(context.Background(), routingAssetCacheConfig(), selectors, clock, load)
		if err != nil || len(data.domains) != len(domains) {
			t.Fatal("uncached projection was incomplete", len(data.domains), err)
		}
		if next := service.routingRefreshAt(); !next.Equal(now.Add(time.Hour)) {
			t.Fatal("uncached verified result lost its refresh deadline", next)
		}
	}
	if calls != 2 || publications != 1 || len(assets.entries) != 1 || len(assets.domains) != 0 || assets.bytes > maxRoutingAssetCacheBytes {
		t.Fatal("oversized result lost metadata or repeated publication", calls, publications, len(assets.entries), len(assets.domains), assets.bytes)
	}
}
