package config

import (
	"fmt"
	"net/netip"
	"strings"
)

// Proxy trust is a bootstrap setting, independent of writeable dashboard data.
// Empty configuration trusts no forwarding headers and never resolves hostnames.
func parseTrustedProxyCIDRs(raw string) ([]netip.Prefix, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 64 {
		return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS supports at most 64 networks")
	}
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil || prefix.Bits() == 0 {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS requires explicit proxy networks narrower than /0")
		}
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS has an ambiguous mapped IPv4 network")
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			if prefix.Bits() == 0 {
				return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS cannot trust every IPv4 address")
			}
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}
