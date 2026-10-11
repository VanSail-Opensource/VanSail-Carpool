package iplookup

import "net/netip"

// CacheKey shares completed IPv4 reports within /24; IPv6 remains exact-only.
func CacheKey(ip string) string {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	address = address.Unmap()
	if address.Is4() {
		return netip.PrefixFrom(address, 24).Masked().String()
	}
	return address.String()
}
