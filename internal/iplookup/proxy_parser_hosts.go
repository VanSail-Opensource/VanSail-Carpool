package iplookup

import "slices"

// ExtractProxyHosts is the bounded compatibility adapter for small documents.
// Large production inventories use readInventory and never build this slice.
func ExtractProxyHosts(data []byte) ([]string, error) {
	hosts := map[string]bool{}
	partial := false
	err := walkInventory(data, func(host string) {
		if len(hosts) >= maxProxyHosts && !hosts[host] {
			partial = true
			return
		}
		hosts[host] = true
	})
	out := make([]string, 0, len(hosts))
	for host := range hosts {
		out = append(out, host)
	}
	slices.Sort(out)
	if partial || err != nil {
		return out, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	return out, nil
}
