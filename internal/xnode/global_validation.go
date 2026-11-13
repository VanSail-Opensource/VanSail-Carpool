package xnode

import (
	"context"
	"io"
	"maps"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func prepareGlobalChange(previous, next GlobalConfig) (GlobalConfig, error) {
	next = cloneGlobal(next)
	next.Telemetry.IncludeActiveIPs = true
	if next.AutomaticWhitelist == nil {
		next.AutomaticWhitelist = previous.AutomaticWhitelist
	}
	for name, provider := range next.Providers {
		old := previous.Providers[name]
		if old.Type == provider.Type {
			merged := maps.Clone(old.Credentials)
			if merged == nil {
				merged = map[string]string{}
			}
			for key, value := range provider.Credentials {
				if value != "" {
					merged[key] = value
				}
			}
			provider.Credentials = merged
		}
		next.Providers[name] = provider
	}
	if err := ValidateGlobal(next); err != nil {
		return GlobalConfig{}, err
	}
	return next, nil
}

func (s *Service) validateGlobalChange(ctx context.Context, previous, next GlobalConfig) error {
	if globalPoliciesKey(previous) != globalPoliciesKey(next) {
		if err := s.validateGlobalPolicies(ctx, next); err != nil {
			return err
		}
	}
	runtimeChanged := globalRuntimeKey(previous) != globalRuntimeKey(next)
	if runtimeChanged {
		if err := s.validateAffected(ctx, nil, nil, &next); err != nil {
			return err
		}
	}
	for _, asset := range []struct {
		target, previous, hash, previousHash string
		geoIP                                bool
	}{
		{next.GeoIPURL, previous.GeoIPURL, next.GeoIPHash, previous.GeoIPHash, true},
		{next.GeoSiteURL, previous.GeoSiteURL, next.GeoSiteHash, previous.GeoSiteHash, false},
	} {
		target := asset.target
		if target == asset.previous && asset.hash == asset.previousHash {
			continue
		}
		if target == "" || asset.geoIP && !requiresAssetLinkProbe(target, next) {
			continue
		}
		// Enabled GeoIP countries were just fetched, hashed, and parsed in
		// validateGlobalPolicies. Repeating a separate Range probe here both
		// wastes a request and can reject hosts that serve full downloads but
		// do not support byte ranges.
		if asset.geoIP && next.SourceGeoIP.Enabled {
			continue
		}
		if err := s.validateAssetLink(ctx, target); err != nil {
			return err
		}
	}
	return nil
}

func requiresAssetLinkProbe(target string, config GlobalConfig) bool {
	return target != "" && !(config.SourceGeoIP.Enabled && target == config.GeoIPURL)
}

func (s *Service) validateAssetLink(ctx context.Context, target string) error {
	return upstreamqueue.Execute(ctx, s.validationQueue, func(ctx context.Context) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return ErrInvalid
		}
		request.Header.Set("Range", "bytes=0-4095")
		response, err := publicAssetClient(15 * time.Second).Do(request)
		if err != nil {
			return assetRequestFailure("asset link", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
			return assetResponseFailure("asset link", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 4096))
		if err != nil || len(data) == 0 {
			return ErrUnavailable
		}
		return nil
	})
}
