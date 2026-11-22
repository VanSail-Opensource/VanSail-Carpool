package xnode

import (
	"context"
	"testing"
	"time"
)

func TestRoutingAssetsPublishPerSourceGeneration(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	publications, firstHash := 0, "first"
	assets := &routingAssets{publish: func() { publications++ }}
	first, second := routingAssetCacheConfig(), routingAssetCacheConfig()
	second.GeoSiteURL = "https://second.example/geosite.dat"
	load := func(_ context.Context, config GlobalConfig, selectors []string) (routingAssetResult, error) {
		hash := firstHash
		if config.GeoSiteURL == second.GeoSiteURL {
			hash = "second"
		}
		return routingAssetFixture(hash, selectors), nil
	}
	read := func(config GlobalConfig, selector string) {
		t.Helper()
		if _, err := assets.resolve(context.Background(), config, []string{selector}, clock, load); err != nil {
			t.Fatal(err)
		}
	}
	read(first, "geosite:google")
	read(second, "geosite:google")
	read(first, "geosite:youtube")
	if publications != 2 {
		t.Fatal("unchanged first source republished after another source", publications)
	}
	now = now.Add(time.Hour)
	firstHash = "second"
	read(first, "geosite:google")
	if publications != 3 {
		t.Fatal("source change was hidden by another source's matching hash", publications)
	}
}
