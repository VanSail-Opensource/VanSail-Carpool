package iplookup

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

func (s *LiveService) shodan(ctx context.Context, ip string) ShodanDetails {
	result := ShodanDetails{Hostnames: []string{}, Ports: []int{}, CPEs: []string{}, Tags: []string{}, Vulns: []string{}}
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
	portsErr, namesErr := json.Unmarshal(o["ports"], &ports), json.Unmarshal(o["hostnames"], &hostnames)
	if portsErr != nil || namesErr != nil || string(o["ports"]) == "null" || string(o["hostnames"]) == "null" {
		result.LiveMeta = liveMeta("shodan:internetdb", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
	}
	// Preserve valid findings even when another field has malformed coverage.
	for _, field := range []struct {
		key   string
		items *[]string
	}{{"cpes", &result.CPEs}, {"tags", &result.Tags}, {"vulns", &result.Vulns}} {
		items, valid := shodanItems(o[field.key])
		*field.items = items
		if !valid && result.Status == "success" {
			result.Status, result.ErrorCode = "partial", "IP_DETAILS_INVALID_RESPONSE"
		}
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
	if len(result.Ports) == 0 && len(result.Hostnames) == 0 && len(result.CPEs) == 0 && len(result.Tags) == 0 && len(result.Vulns) == 0 && result.Status == "success" {
		result.Status = "empty"
	}
	return result
}

func shodanItems(raw json.RawMessage) ([]string, bool) {
	items := []string{}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return items, false
	}
	valid, seen := true, map[string]bool{}
	for _, value := range values {
		var item string
		if json.Unmarshal(value, &item) != nil || strings.TrimSpace(item) == "" {
			valid = false
			continue
		}
		item = strings.TrimSpace(item)
		if !seen[item] {
			items = append(items, item)
			seen[item] = true
		}
	}
	sort.Strings(items)
	return items, valid
}
