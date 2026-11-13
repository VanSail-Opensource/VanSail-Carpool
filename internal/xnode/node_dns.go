package xnode

import "encoding/json"

// Node-scoped native defaults belong to stored state. Public edits cannot erase
// them, and explicit native DNS always takes priority over a retained default.
func (s *Service) preserveNodeDNS(previous Node, settings *NodeSettings) error {
	if previous.SettingsCipher == "" {
		return nil
	}
	old, err := s.openNodeSettings(previous)
	if err != nil {
		return err
	}
	settings.dnsFallbackServers = append([]string(nil), old.dnsFallbackServers...)
	return nil
}

func applyNodeDNS(native map[string]any, settings NodeSettings) {
	if !settings.DNS.Redirect || len(settings.dnsFallbackServers) == 0 {
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
	servers := make([]any, len(settings.dnsFallbackServers))
	for index, server := range settings.dnsFallbackServers {
		servers[index] = server
	}
	resolver["servers"] = servers
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
