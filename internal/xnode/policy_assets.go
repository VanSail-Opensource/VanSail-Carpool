package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"sync"
	"time"
)

type policyAssets struct {
	mu              sync.Mutex
	key, generation string
	scheduleKey     string
	completedAt     time.Time
	prefixes        []string
	next            time.Time
	failures        int
	publish         func()
}

func (s *Service) sourceGeoIP(ctx context.Context, config GlobalConfig, firewall wire.FirewallPolicy, native json.RawMessage) (*wire.SourceGeoIPPolicy, error) {
	if !config.SourceGeoIP.Enabled || !firewall.Enabled {
		return nil, nil
	}
	listeners, err := wire.SourceGeoIPListeners(native)
	if err != nil {
		return nil, err
	}
	assets, err := s.resolveGeoIPCached(ctx, config)
	if err != nil {
		return nil, err
	}
	return &wire.SourceGeoIPPolicy{Generation: assets.generation, Prefixes: assets.prefixes, Listeners: listeners}, nil
}

func (s *Service) resolveGeoIPCached(ctx context.Context, config GlobalConfig) (geoIPAssets, error) {
	assets := s.policyAssets
	assets.mu.Lock()
	defer assets.mu.Unlock()
	raw, _ := json.Marshal(struct {
		URL, Hash string
		Selectors []string
	}{config.GeoIPURL, config.GeoIPHash, config.SourceGeoIP.Selectors})
	key := string(raw)
	now := s.now()
	scheduleRaw, _ := json.Marshal(struct {
		Hours *int
		Cron  string
	}{config.GeodataIntervalHours, config.GeodataCron})
	if assets.scheduleKey != string(scheduleRaw) {
		assets.scheduleKey = string(scheduleRaw)
		assets.failures = 0
		if assets.key == key && assets.generation != "" {
			completed := assets.completedAt
			if completed.IsZero() {
				completed = now
			}
			assets.next = nextMasterGeodata(config, completed)
		}
	}
	if assets.key != key || assets.generation == "" || !assets.next.IsZero() && !now.Before(assets.next) {
		prefixes, generation, err := s.resolveGeoIP(ctx, config)
		completed := s.now()
		next := nextMasterGeodata(config, completed)
		if err != nil {
			if assets.key != key || assets.generation == "" {
				return geoIPAssets{}, err
			}
			if !next.IsZero() {
				retries := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}
				if assets.failures < len(retries) {
					next = completed.Add(retries[assets.failures])
					assets.failures++
				} else {
					assets.failures = 0
				}
			}
		} else {
			changed := assets.generation != generation
			assets.key, assets.prefixes, assets.generation, assets.failures = key, prefixes, generation, 0
			if changed {
				assets.publish()
			}
		}
		assets.next, assets.completedAt = next, completed
	}
	return geoIPAssets{generation: assets.generation, prefixes: assets.prefixes}, nil
}

type geoIPAssets struct {
	generation string
	prefixes   []string
}

func (s *Service) policyRefreshAt() time.Time {
	if s.policyAssets == nil {
		return time.Time{}
	}
	s.policyAssets.mu.Lock()
	defer s.policyAssets.mu.Unlock()
	return s.policyAssets.next
}

func (s *Service) invalidatePolicyAssets() {
	s.policyAssets.mu.Lock()
	s.policyAssets.next = s.now().Add(-time.Second)
	s.policyAssets.mu.Unlock()
	s.localCompileRevision.Add(1)
}
