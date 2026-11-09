package xnode

import "encoding/json"

// Legacy state is interpreted once at the settings boundary. The compatibility
// marker remains node-owned; shared native configurations are never rewritten.
func normalizeNodeDNS(settings *NodeSettings) {
	if settings.DNS.Mode == "smartdns" && settings.DNS.Redirect {
		settings.LegacyDNSLoopback = true
	}
	settings.DNS.Mode = "xray"
}

// Older clients do not know the marker. An unrelated node save must not erase
// its previously configured upstream; explicit native DNS still takes priority.
func (s *Service) preserveNodeDNS(previous Node, settings *NodeSettings) error {
	if settings.LegacyDNSLoopback || previous.SettingsCipher == "" {
		return nil
	}
	var old NodeSettings
	if err := s.open("xnode.node."+previous.ID, previous.SettingsCipher, &old); err != nil {
		return err
	}
	normalizeNodeDNS(&old)
	settings.LegacyDNSLoopback = old.LegacyDNSLoopback
	return nil
}

func applyNodeDNS(native map[string]any, settings NodeSettings) {
	if !settings.DNS.Redirect || !settings.LegacyDNSLoopback {
		return
	}
	resolver, ok := native["dns"].(map[string]any)
	if !ok {
		if native["dns"] != nil {
			return // Preserve malformed input for the native validator.
		}
		resolver = map[string]any{}
	}
	if servers := resolver["servers"]; servers != nil {
		values, ok := servers.([]any)
		if !ok || len(values) > 0 {
			return
		}
	}
	resolver["servers"] = []any{"127.0.0.1"}
	native["dns"] = resolver
}

func nodeDNSDocument(raw json.RawMessage, settings NodeSettings) (json.RawMessage, error) {
	var native map[string]any
	if json.Unmarshal(raw, &native) != nil || native == nil {
		return nil, ErrInvalid
	}
	applyNodeDNS(native, settings)
	return json.Marshal(native)
}
