package xnode

import "encoding/json"

// Retired fields are accepted only when reading encrypted historical settings.
// The request type stays strict, and subsequent saves omit the retired policy.
func (s *Service) openNodeSettings(node Node) (NodeSettings, error) {
	var settings NodeSettings
	stored := struct {
		*NodeSettings
		RetiredBandwidth json.RawMessage `json:"bandwidth"`
	}{NodeSettings: &settings}
	err := s.open("xnode.node."+node.ID, node.SettingsCipher, &stored)
	return settings, err
}
