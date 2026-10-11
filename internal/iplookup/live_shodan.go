package iplookup

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

func (s *LiveService) shodan(ctx context.Context, ip string) ShodanDetails {
	result := ShodanDetails{Hostnames: []string{}, Ports: []int{}}
	data, err := s.liveHTTP(ctx, "shodan", "/"+url.PathEscape(ip), nil, nil, "")
	result.LiveMeta = liveMeta("shodan:internetdb", err)
	if ErrorCode(err) == "IP_DETAILS_NOT_FOUND" {
		result.LiveMeta = LiveMeta{Status: "empty", Source: "shodan:internetdb"}
		return result
	}
	if err != nil {
		return result
	}
	o := liveObject(data)
	if !validReturnedIP(o, "ip", ip) {
		result.LiveMeta = liveMeta("shodan:internetdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	var ports []int
	var hostnames []string
	if json.Unmarshal(o["ports"], &ports) != nil || json.Unmarshal(o["hostnames"], &hostnames) != nil || string(o["ports"]) == "null" || string(o["hostnames"]) == "null" {
		result.LiveMeta = liveMeta("shodan:internetdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	seenPorts, seenNames := map[int]bool{}, map[string]bool{}
	for _, port := range ports {
		if port >= 1 && port <= 65535 {
			seenPorts[port] = true
		} else {
			result.Status = "partial"
		}
	}
	for _, name := range hostnames {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".")
		if len(name) > 0 && len(name) <= 253 {
			seenNames[name] = true
		} else {
			result.Status = "partial"
		}
	}
	for port := range seenPorts {
		result.Ports = append(result.Ports, port)
	}
	for name := range seenNames {
		result.Hostnames = append(result.Hostnames, name)
	}
	sort.Ints(result.Ports)
	sort.Strings(result.Hostnames)
	if len(result.Hostnames) > 256 {
		result.Hostnames = result.Hostnames[:256]
		result.Status = "partial"
	}
	// InternetDB does not expose a per-host observation timestamp.
	if len(result.Ports) == 0 && len(result.Hostnames) == 0 && result.Status == "success" {
		result.Status = "empty"
	}
	return result
}
