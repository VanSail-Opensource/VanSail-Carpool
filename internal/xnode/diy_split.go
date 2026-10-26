package xnode

import (
	"fmt"
	"slices"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type DIYSplitHost struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	NodeID          string                   `json:"nodeId"`
	InboundTag      string                   `json:"inboundTag"`
	DownloadIDs     []string                 `json:"downloadIds"`
	Modes           []string                 `json:"modes"`
	DownloadOptions []DIYSplitDownloadOption `json:"downloadOptions,omitempty"`
}

type DIYSplitDownloadOption struct {
	DownloadEndpointID string   `json:"downloadEndpointId"`
	Modes              []string `json:"modes"`
}

func splitHostCatalog(proxies []subscription.Proxy) []DIYSplitHost {
	// Catalog current administrator mappings once per host, without credentials
	// in the returned metadata or repeated mapping work for every possible pair.
	mapped := make([]subscription.Proxy, 0, len(proxies))
	original := make(map[string]subscription.Proxy, len(proxies))
	for _, p := range proxies {
		original[p.ID] = p
		p.Tuning = nil
		out, err := p.Outbound()
		if err != nil || out["protocol"] != p.Protocol {
			continue
		}
		p.Stream, _ = out["streamSettings"].(map[string]any)
		mapped = append(mapped, p)
	}
	proxies = mapped
	result := []DIYSplitHost{}
	for _, up := range proxies {
		host := DIYSplitHost{ID: up.ID, Name: up.Name, NodeID: up.NodeID, InboundTag: up.InboundTag, DownloadIDs: []string{}, Modes: []string{}, DownloadOptions: []DIYSplitDownloadOption{}}
		for _, down := range proxies {
			modes := subscription.SplitModes(up, down)
			rawModes := subscription.SplitModes(original[up.ID], original[down.ID])
			modes = slices.DeleteFunc(modes, func(mode string) bool { return !slices.Contains(rawModes, mode) })
			if len(modes) > 0 {
				host.DownloadIDs = append(host.DownloadIDs, down.ID)
				host.DownloadOptions = append(host.DownloadOptions, DIYSplitDownloadOption{DownloadEndpointID: down.ID, Modes: modes})
				for _, mode := range modes {
					if !slices.Contains(host.Modes, mode) {
						host.Modes = append(host.Modes, mode)
					}
				}
			}
		}
		if len(host.DownloadIDs) > 0 {
			result = append(result, host)
		}
	}
	return result
}

func validSplitProfiles(profiles []SplitProfile, proxies []subscription.Proxy) []SplitProfile {
	byID := map[string]subscription.Proxy{}
	for _, p := range proxies {
		byID[p.ID] = p
	}
	result := []SplitProfile{}
	for _, profile := range profiles {
		if _, err := subscription.SplitProxy(profile.ID, profile.Mode, byID[profile.UploadEndpointID], byID[profile.DownloadEndpointID]); err == nil {
			result = append(result, profile)
		}
	}
	return result
}

func validateSplitProfiles(profiles []SplitProfile, proxies []subscription.Proxy) error {
	if len(profiles) > 64 || len(validSplitProfiles(profiles, proxies)) != len(profiles) {
		return ErrInvalid
	}
	ids, pairs := map[string]bool{}, map[[2]string]bool{}
	for _, profile := range profiles {
		pair := [2]string{profile.UploadEndpointID, profile.DownloadEndpointID}
		if !slices.Contains([]string{"auto", "packet-up", "stream-up"}, profile.Mode) || profile.ID == "" || len(profile.ID) > 128 || strings.Trim(profile.ID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" || ids[profile.ID] || pairs[pair] {
			return ErrInvalid
		}
		ids[profile.ID], pairs[pair] = true, true
	}
	return nil
}

func appendSplitProxies(profiles []SplitProfile, proxies []subscription.Proxy) []subscription.Proxy {
	result := slices.Clone(proxies)
	byID := map[string]subscription.Proxy{}
	names := map[string]bool{}
	for _, p := range proxies {
		byID[p.ID] = p
		names[p.Name] = true
	}
	ordered := slices.Clone(profiles)
	slices.SortFunc(ordered, func(a, b SplitProfile) int { return strings.Compare(a.ID, b.ID) })
	for _, profile := range ordered {
		combined, err := subscription.SplitProxy(profile.ID, profile.Mode, byID[profile.UploadEndpointID], byID[profile.DownloadEndpointID])
		if err != nil {
			continue // A stale combined projection never removes ordinary hosts.
		}
		// Reverse pairs share the same compact name. Preserve ordinary names
		// and disambiguate only the colliding projection, without changing IDs.
		base := combined.Name
		for n := 2; names[combined.Name]; n++ {
			combined.Name = fmt.Sprintf("%s (%d)", base, n)
		}
		names[combined.Name] = true
		result = append(result, combined)
	}
	return result
}
