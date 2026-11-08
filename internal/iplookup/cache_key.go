package iplookup

import "net/netip"

// CacheKey isolates completed evidence by canonical address.
func CacheKey(ip string) string {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	address = address.Unmap()
	return address.String()
}
