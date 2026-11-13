package xnode

import (
	"encoding/json"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Only encrypted historical settings use this adapter. Request decoding uses
// NodeSettings directly and rejects every retired field below.
type historicalNodeSettings struct {
	NodeSettings
	NativeDNSFallbackServers *[]string                            `json:"nativeDnsFallbackServers"`
	LegacyDNSLoopback        bool                                 `json:"legacyDnsLoopback"`
	DDNSBindings             map[string]historicalDDNSHostBinding `json:"ddnsBindings"`
	DNS                      struct {
		Mode     string `json:"mode"`
		Redirect bool   `json:"redirect"`
	} `json:"dns"`
}

func (stored historicalNodeSettings) current() (NodeSettings, error) {
	settings := stored.NodeSettings
	settings.DNS = wire.DNSPolicy{Redirect: stored.DNS.Redirect}
	if stored.NativeDNSFallbackServers != nil {
		// This private field contains only the established node-local default.
		// It cannot become an unvalidated arbitrary resolver override.
		servers := *stored.NativeDNSFallbackServers
		if len(servers) > 1 || len(servers) == 1 && servers[0] != "127.0.0.1" {
			return NodeSettings{}, ErrInvalid
		}
		settings.dnsFallbackServers = append([]string(nil), servers...)
	} else if stored.LegacyDNSLoopback || stored.DNS.Mode == "smartdns" && stored.DNS.Redirect {
		settings.dnsFallbackServers = []string{"127.0.0.1"}
	}
	if stored.DDNSBindings != nil {
		settings.DDNSBindings = make(map[string]DDNSHostBinding, len(stored.DDNSBindings))
		for id, historical := range stored.DDNSBindings {
			binding, err := historical.current()
			if err != nil {
				return NodeSettings{}, err
			}
			settings.DDNSBindings[id] = binding
		}
	}
	return settings, nil
}

type historicalDDNSHostBinding struct {
	IPv4HostIDs json.RawMessage `json:"ipv4HostIds"`
	IPv6HostIDs json.RawMessage `json:"ipv6HostIds"`
	IPv4HostID  string          `json:"ipv4HostId"`
	IPv6HostID  string          `json:"ipv6HostId"`
}

func (stored historicalDDNSHostBinding) current() (DDNSHostBinding, error) {
	var binding DDNSHostBinding
	for _, family := range []struct {
		current json.RawMessage
		former  string
		target  *[]string
	}{
		{stored.IPv4HostIDs, stored.IPv4HostID, &binding.IPv4HostIDs},
		{stored.IPv6HostIDs, stored.IPv6HostID, &binding.IPv6HostIDs},
	} {
		if family.current != nil {
			// An explicitly empty or null array never resurrects a former host.
			if err := json.Unmarshal(family.current, family.target); err != nil {
				return DDNSHostBinding{}, err
			}
		} else if family.former != "" {
			*family.target = []string{family.former}
		}
	}
	return binding, nil
}
