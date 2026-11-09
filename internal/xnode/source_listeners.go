package xnode

import (
	"encoding/json"
	"errors"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// sourceGeoIPListeners derives the country source-filter bindings from a
// document with Hysteria inbounds removed. Hysteria clients are commonly
// direct Chinese broadband users and would otherwise be silently dropped by
// the country rule; they rely on the node's probe protection (protected input
// policy, invalid-packet and Xray authentication detection) instead.
func sourceGeoIPListeners(native json.RawMessage) ([]wire.SourceGeoIPListener, error) {
	filtered, err := withoutHysteriaInbounds(native)
	if err != nil {
		return nil, err
	}
	return wire.SourceGeoIPListeners(filtered)
}

func withoutHysteriaInbounds(native json.RawMessage) (json.RawMessage, error) {
	var root map[string]any
	if err := json.Unmarshal(native, &root); err != nil {
		return nil, err
	}
	raw, exists := root["inbounds"]
	if !exists {
		return native, nil
	}
	inbounds, ok := raw.([]any)
	if !ok {
		return nil, errors.New("invalid inbound list")
	}
	kept := make([]any, 0, len(inbounds))
	for _, entry := range inbounds {
		inbound, _ := entry.(map[string]any)
		if protocol, _ := inbound["protocol"].(string); protocol == "hysteria" {
			continue
		}
		kept = append(kept, entry)
	}
	root["inbounds"] = kept
	return json.Marshal(root)
}
