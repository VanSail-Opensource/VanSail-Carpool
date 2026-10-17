package xnode

import "encoding/json"

// ValidateClientConfigurationDocument keeps administrator client documents
// separate from the agent's stricter IP-geodata policy. Analysis substitutes
// client-local assets without mutating or downloading the actual client document.
func ValidateClientConfigurationDocument(raw json.RawMessage) ValidationResult {
	var native map[string]any
	if json.Unmarshal(raw, &native) != nil {
		return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
	}
	local := nodeLocalAssets(native)
	if local {
		analysis, err := validationAssets(native)
		if err != nil {
			return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
		}
		raw, err = json.Marshal(analysis)
		if err != nil {
			return ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
		}
	}
	result := ValidateConfigurationDocument(raw)
	result.NeedsAgentValidation = result.Valid && local
	return result
}
