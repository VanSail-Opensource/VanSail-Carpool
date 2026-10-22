package xnode

import (
	"slices"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type DIYSplitHost struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	NodeID      string   `json:"nodeId"`
	InboundTag  string   `json:"inboundTag"`
	DownloadIDs []string `json:"downloadIds"`
	Modes       []string `json:"modes"`
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
		host := DIYSplitHost{ID: up.ID, Name: up.Name, NodeID: up.NodeID, InboundTag: up.InboundTag, DownloadIDs: []string{}, Modes: []string{}}
		for _, down := range proxies {
			modes := subscription.SplitModes(up, down)
			rawModes := subscription.SplitModes(original[up.ID], original[down.ID])
			modes = slices.DeleteFunc(modes, func(mode string) bool { return !slices.Contains(rawModes, mode) })
			if len(modes) > 0 {
				host.DownloadIDs = append(host.DownloadIDs, down.ID)
				host.Modes = modes
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
	for _, p := range proxies {
		byID[p.ID] = p
	}
	for _, profile := range profiles {
		combined, err := subscription.SplitProxy(profile.ID, profile.Mode, byID[profile.UploadEndpointID], byID[profile.DownloadEndpointID])
		if err != nil {
			continue // A stale combined projection never removes ordinary hosts.
		}
		result = append(result, combined)
	}
	return result
}
