package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
)

type routingAssetSettings struct {
	cacheTestSettings
	raw string
}

func (s routingAssetSettings) Optional(context.Context, string) (string, error) { return s.raw, nil }

func TestSelectedGeoSiteFailuresAndStaleCache(t *testing.T) {
	for _, address := range []string{"", "https://unreachable.invalid/geosite.dat"} {
		t.Run(address, func(t *testing.T) {
			config := DefaultGlobal()
			config.GeoSiteURL = address
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "routing-assets", Capacity: 2})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := queue.Start(ctx); err != nil {
				t.Fatal(err)
			}
			defer queue.Shutdown(context.Background())
			s := NewService(nil, routingAssetSettings{raw: string(raw)}, nil, nil, queue)
			templates := []RoutingTemplate{{ID: "selected", Match: routingmatch.Match{Domains: []string{"geosite:google"}}}}
			_, _, err = s.routingExpansion(ctx, templates)
			want := "XNODE_GEOSITE_UNAVAILABLE"
			if address == "" {
				want = "XNODE_GEOSITE_REQUIRED"
			}
			var policy *PolicyError
			if !errors.As(err, &policy) || policy.Code != want {
				t.Fatal(err, want)
			}
			if address == "" {
				if !errors.Is(err, ErrInvalid) {
					t.Fatal(err)
				}
				return
			}
			key, _ := json.Marshal([]any{config.GeoSiteURL, config.GeoSiteHash, []string{"geosite:google"}, config.GeodataIntervalHours, config.GeodataCron})
			s.routingAssets.key = string(key)
			s.routingAssets.data = routingAssetResult{hash: "previous", domains: map[string][]string{"geosite:google": {"domain:google.com"}}}
			s.routingAssets.next = time.Now().Add(-time.Minute)
			domains, hash, err := s.routingExpansion(ctx, templates)
			if err != nil || hash != "previous" || len(domains["geosite:google"]) != 1 {
				t.Fatal("refresh discarded stale verified data", err)
			}
		})
	}
}

func TestGeoSiteDataFailuresHaveSpecificPolicyCode(t *testing.T) {
	for _, failure := range []error{ErrInvalid, assetDecodeFailure(errors.New("truncated asset"))} {
		var policy *PolicyError
		if err := geoSitePolicyFailure(failure); !errors.As(err, &policy) || policy.Code != "XNODE_DOMAIN_SELECTOR_INVALID" || !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	// No choices never need settings, a running queue or GeoSite availability.
	s := &Service{}
	if _, hash, err := s.routingExpansion(context.Background(), nil); err != nil || hash != "" {
		t.Fatal(err, hash)
	}
}
