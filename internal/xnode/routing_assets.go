package xnode

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"
)

type routingAssets struct {
	mu       sync.Mutex
	key      string
	data     routingAssetResult
	next     time.Time
	failures int
	publish  func()
}

func (s *Service) routingExpansion(ctx context.Context, templates []RoutingTemplate) (map[string][]string, string, error) {
	selectors := []string{}
	for _, t := range templates {
		for _, d := range t.Match.Domains {
			if strings.HasPrefix(d, "geosite:") && !slices.Contains(selectors, d) {
				selectors = append(selectors, d)
			}
		}
	}
	slices.Sort(selectors)
	if len(selectors) == 0 {
		return map[string][]string{}, "", nil
	}
	config, err := s.Global(ctx)
	if err != nil {
		return nil, "", err
	}
	key, _ := json.Marshal([]any{config.GeoSiteURL, config.GeoSiteHash, selectors, config.GeodataIntervalHours, config.GeodataCron})
	a := s.routingAssets
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.key != string(key) || a.data.hash == "" || !a.next.IsZero() && !s.now().Before(a.next) {
		data, err := s.downloadRoutingGeoSite(ctx, config, selectors)
		completed := s.now()
		next := nextMasterGeodata(config, completed)
		if err != nil {
			if a.key != string(key) || a.data.hash == "" {
				return nil, "", err
			}
			retries := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}
			if !next.IsZero() && a.failures < len(retries) {
				next = completed.Add(retries[a.failures])
				a.failures++
			} else {
				a.failures = 0
			}
		} else {
			changed := a.data.hash != data.hash
			a.data = data
			a.key = string(key)
			a.failures = 0
			if changed {
				a.publish()
			}
		}
		a.next = next
	}
	return a.data.domains, a.data.hash, nil
}
func (s *Service) routingRefreshAt() time.Time {
	s.routingAssets.mu.Lock()
	defer s.routingAssets.mu.Unlock()
	next := s.routingAssets.next
	if !next.IsZero() && !s.now().Before(next) {
		return time.Time{}
	}
	return next
}
