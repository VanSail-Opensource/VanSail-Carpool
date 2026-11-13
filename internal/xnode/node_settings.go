package xnode

import (
	"bytes"
	"encoding/json"
	"errors"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// errNodeSettingsNotObject rejects a stored document that is valid JSON but not
// a settings object. json.Unmarshal alone would accept JSON null as an empty
// document, which must not be mistaken for historical settings evidence.
var errNodeSettingsNotObject = errors.New("stored node settings are not a JSON object")

// decodeNodeSettings reads one historical encrypted settings document. Stored
// settings are historical evidence, not a request body: a policy field retired by
// a later release must not make the whole node undecodable (every firewall,
// protected-bank and DDNS job reads these settings). Unknown stored keys are
// therefore ignored on read, while malformed documents, non-object documents and
// wrong-typed known fields are rejected. The SaveNode request type and its
// validation stay strict so new writes cannot persist unsupported fields.
func decodeNodeSettings(raw []byte) (NodeSettings, error) {
	if len(raw) > wire.MaxFrameBytes {
		return NodeSettings{}, ErrInvalid
	}
	document := bytes.TrimSpace(raw)
	if len(document) == 0 || document[0] != '{' {
		return NodeSettings{}, errNodeSettingsNotObject
	}
	var stored historicalNodeSettings
	if err := json.Unmarshal(document, &stored); err != nil {
		return NodeSettings{}, err
	}
	return stored.current()
}

// Persist current explicit native defaults separately from the public settings
// shape. The next ordinary save removes retired keys; reads never rewrite state.
func (s *Service) sealNodeSettings(node string, settings NodeSettings) (string, error) {
	stored := struct {
		NodeSettings
		NativeDNSFallbackServers []string `json:"nativeDnsFallbackServers,omitempty"`
	}{settings, settings.dnsFallbackServers}
	return s.seal("xnode.node."+node, stored)
}

func (s *Service) openNodeSettings(node Node) (NodeSettings, error) {
	raw, err := s.vault.Decrypt("xnode.node."+node.ID, node.SettingsCipher)
	if err != nil {
		return NodeSettings{}, err
	}
	return decodeNodeSettings([]byte(raw))
}
