package xnode

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// Client mappers may shape transport, but cannot replace private authentication,
// redirect a managed hop or introduce an unmodelled dialer-proxy dependency.
func guardedRoutingOutbound(proxy subscription.Proxy) (map[string]any, error) {
	unknown := func() (map[string]any, error) {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, &routingmatch.Diagnostic{Code: "routing_context_unproven"})
	}
	out, err := proxy.Outbound()
	if err != nil {
		return nil, err
	}
	baseline := proxy
	baseline.MapperHost = nil
	expected, err := baseline.Outbound()
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"protocol", "settings"} {
		actualJSON, err := json.Marshal(out[key])
		if err != nil {
			return nil, err
		}
		expectedJSON, err := json.Marshal(expected[key])
		if err != nil {
			return nil, err
		}
		if string(actualJSON) != string(expectedJSON) {
			return unknown()
		}
	}
	if out["proxySettings"] != nil {
		return unknown()
	}
	stream, _ := out["streamSettings"].(map[string]any)
	sockopt, _ := stream["sockopt"].(map[string]any)
	if value := sockopt["dialerProxy"]; value != nil && value != "" {
		return unknown()
	}
	// Client exports explicitly disable MPTCP. A managed hop uses that same
	// native default without introducing a socket override into the agent guard.
	// Retain every other socket field so the native permission check still sees it.
	if sockopt["tcpMptcp"] == false {
		out, stream, sockopt = maps.Clone(out), maps.Clone(stream), maps.Clone(sockopt)
		delete(sockopt, "tcpMptcp")
		if len(sockopt) == 0 {
			delete(stream, "sockopt")
		} else {
			stream["sockopt"] = sockopt
		}
		out["streamSettings"] = stream
	}
	return out, nil
}
