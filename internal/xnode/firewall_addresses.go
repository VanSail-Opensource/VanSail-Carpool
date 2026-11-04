package xnode

import (
	"net/netip"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func FirewallPublicIP(raw string) (string, error) {
	ip, err := firewallPublicAddr(raw)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

func firewallPublicAddr(raw string) (netip.Addr, error) {
	ip, err := wire.AutomaticLeaseAddress(raw)
	if err != nil {
		return netip.Addr{}, ErrInvalid
	}
	return ip, nil
}
