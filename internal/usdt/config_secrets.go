package usdt

import "slices"

// mergeSecrets resolves masked URLs before deciding whether a key can be reused.
// Network identity and all destinations are part of the credential boundary;
// an endpoint ID alone never authorizes inheritance. Rejection leaves drafts intact.
func mergeSecrets(next *Config, previous Config) error {
	type endpointKey struct{ network, id string }
	old := map[endpointKey]Endpoint{}
	for _, network := range previous.Networks {
		spec, ok := Network(network.Network)
		if !ok {
			return ErrConfiguration
		}
		for _, endpoint := range network.Endpoints {
			old[endpointKey{spec.Network, endpoint.ID}] = endpoint
		}
	}
	networks := slices.Clone(next.Networks)
	for i := range networks {
		network := &networks[i]
		spec, ok := Network(network.Network)
		if !ok {
			return ErrConfiguration
		}
		network.Endpoints = slices.Clone(network.Endpoints)
		for j := range network.Endpoints {
			endpoint := &network.Endpoints[j]
			before := old[endpointKey{spec.Network, endpoint.ID}]
			if endpoint.RPCURL == "" {
				endpoint.RPCURL = before.RPCURL
			}
			if endpoint.WSURL == "" {
				endpoint.WSURL = before.WSURL
			}
			if endpoint.IndexerURL == "" {
				endpoint.IndexerURL = before.IndexerURL
			}
			if endpoint.APIKey == "" {
				if before.APIKey != "" && !sameEndpointDestinations(*endpoint, before) {
					return ErrConfiguration
				}
				endpoint.APIKey = before.APIKey
			}
			endpoint.RPCConfigured, endpoint.WSConfigured = false, false
			endpoint.IndexerConfigured, endpoint.KeyConfigured, endpoint.Host = false, false, ""
		}
	}
	next.Networks = networks
	return nil
}

func sameEndpointDestinations(next, before Endpoint) bool {
	// Include inactive stored destinations: enabling WS later must not expose a
	// key that was inherited while the new destination was temporarily disabled.
	return next.RPCURL == before.RPCURL && next.WSURL == before.WSURL && next.IndexerURL == before.IndexerURL
}
