package xnode

import "net/netip"

var firewallGlobalIPv6 = netip.MustParsePrefix("2000::/3")
var firewallNonPublicPrefixes = func() []netip.Prefix {
	result := []netip.Prefix{}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"} {
		result = append(result, netip.MustParsePrefix(raw))
	}
	return result
}()

func FirewallPublicIP(raw string) (string, error) {
	ip, err := firewallPublicAddr(raw)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

func firewallPublicAddr(raw string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(raw)
	if err != nil || ip.Zone() != "" {
		return netip.Addr{}, ErrInvalid
	}
	ip = ip.Unmap()
	if ip.Is6() && !firewallGlobalIPv6.Contains(ip) {
		return netip.Addr{}, ErrInvalid
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return netip.Addr{}, ErrInvalid
	}
	// Shared address space, documentation and benchmarking ranges are not public.
	for _, prefix := range firewallNonPublicPrefixes {
		if prefix.Contains(ip) {
			return netip.Addr{}, ErrInvalid
		}
	}
	return ip, nil
}
