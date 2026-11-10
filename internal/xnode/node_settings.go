package xnode

import "encoding/json"

// decodeNodeSettings reads one historical encrypted settings document. Stored
// settings are historical evidence, not a request body: a policy field retired by
// a later release must not make the whole node undecodable (every firewall,
// protected-bank and DDNS job reads these settings). Unknown stored keys are
// therefore ignored on read, while the SaveNode request type and its validation
// stay strict so new writes cannot persist unsupported fields.
func decodeNodeSettings(raw []byte) (NodeSettings, error) {
	var settings NodeSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return NodeSettings{}, err
	}
	return settings, nil
}

func (s *Service) openNodeSettings(node Node) (NodeSettings, error) {
	raw, err := s.vault.Decrypt("xnode.node."+node.ID, node.SettingsCipher)
	if err != nil {
		return NodeSettings{}, err
	}
	return decodeNodeSettings([]byte(raw))
}
