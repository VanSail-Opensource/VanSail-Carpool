package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

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
	if config.GeoSiteURL == "" {
		return nil, "", &PolicyError{"XNODE_GEOSITE_REQUIRED", ErrInvalid}
	}
	data, err := s.routingAssets.resolve(ctx, config, selectors, s.now, s.downloadRoutingGeoSite)
	if err != nil {
		return nil, "", geoSitePolicyFailure(err)
	}
	return data.domains, data.hash, nil
}

func routingAssetKeys(config GlobalConfig, selectors []string) (string, string) {
	source, _ := json.Marshal([]any{config.GeoSiteURL, config.GeoSiteHash})
	key, _ := json.Marshal([]any{string(source), selectors, config.GeodataIntervalHours, config.GeodataCron})
	sourceHash, keyHash := sha256.Sum256(source), sha256.Sum256(key)
	return hex.EncodeToString(sourceHash[:]), hex.EncodeToString(keyHash[:])
}

func (s *Service) routingRefreshAt() time.Time {
	a := s.routingAssets
	a.mu.Lock()
	defer a.mu.Unlock()
	now := s.now()
	var next time.Time
	for _, entry := range a.entries {
		if now.Before(entry.next) && (next.IsZero() || entry.next.Before(next)) {
			next = entry.next
		}
	}
	return next
}
