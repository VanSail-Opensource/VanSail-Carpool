package xnode

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (s *Service) SaveGlobal(ctx context.Context, actor string, next GlobalConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.Global(ctx)
	if err != nil {
		return err
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
	if err = ValidateGlobal(next); err != nil {
		return err
	}
	if globalPoliciesKey(previous) != globalPoliciesKey(next) {
		if err = s.validateGlobalPolicies(ctx, next); err != nil {
			return err
		}
	}
	runtimeChanged := globalRuntimeKey(previous) != globalRuntimeKey(next)
	if runtimeChanged {
		if err = s.validateAffected(ctx, nil, nil, &next); err != nil {
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
		if err = s.validateAssetLink(ctx, target); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err = s.settings.PutXNodeGlobal(ctx, actor, string(raw)); err != nil {
		return err
	}
	if repo, ok := s.repo.(interface{ ResetFirewallDetection() }); ok {
		before, _ := json.Marshal(previous.FirewallDetection)
		after, _ := json.Marshal(next.FirewallDetection)
		if string(before) != string(after) {
			repo.ResetFirewallDetection()
		}
	}
	if runtimeChanged {
		s.localCompileRevision.Add(1)
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
