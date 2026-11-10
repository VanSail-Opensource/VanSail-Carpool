package xnode

import (
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/VanSail-Opensource/X-Node/pkg/xrayconfig"
	"strings"
)

func ValidateConfigurationDocument(raw json.RawMessage) ValidationResult {
	return validateConfigurationDocument(raw, nil)
}

func validateConfigurationDocument(raw json.RawMessage, validator *xrayconfig.Validator) ValidationResult {
	invalid := ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}
	var native map[string]any
	if json.Unmarshal(raw, &native) != nil || native == nil {
		return invalid
	}
	// A VLESS reverse portal declares the administrator-owned outbound tag its
	// bridge accounts register. Only one portal per document may own a tag, and
	// the name must not shadow a static outbound the runtime would resolve first.
	reverseTags := map[string]bool{}
	if inbounds, ok := native["inbounds"].([]any); ok {
		for _, raw := range inbounds {
			inbound, _ := raw.(map[string]any)
			if inbound["protocol"] == "shadowsocks" {
				settings, _ := inbound["settings"].(map[string]any)
				method := settings["method"]
				if method != "chacha20-ietf-poly1305" && method != "2022-blake3-aes-128-gcm" && method != "2022-blake3-aes-256-gcm" {
					return invalid
				}
			}
			if value, exists := inbound["reverseTag"]; exists {
				tag, ok := value.(string)
				if !ok || !wire.ValidName(tag) || inbound["protocol"] != "vless" || reverseTags[tag] {
					return invalid
				}
				reverseTags[tag] = true
			}
		}
	}
	outs, ok := native["outbounds"].([]any)
	if !ok {
		return invalid
	}
	outs, hasOuts, err := expandMarker(outs, "UserOutbound", nil)
	if err != nil {
		return invalid
	}
	for _, raw := range outs {
		out, _ := raw.(map[string]any)
		if tag, _ := out["tag"].(string); reverseTags[tag] {
			return invalid
		}
	}
	native["outbounds"] = outs
	routing, _ := native["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)
	if rules == nil {
		rules = []any{}
	}
	rules, hasRules, err := expandMarker(rules, "UserRouting", nil)
	if err != nil || hasRules != hasOuts {
		return invalid
	}
	routing["rules"] = rules
	native["routing"] = routing
	data, err := json.Marshal(native)
	if err != nil {
		return invalid
	}
	config := wire.Config{Xray: data, Users: []wire.User{}}
	if err = config.Validate(); err != nil {
		invalid.Detail = validationDetail(err)
		return invalid
	}
	needsAgent := nodeLocalAssets(native)
	if needsAgent {
		copy, assetErr := validationAssets(native)
		if assetErr != nil {
			return invalid
		}
		clean, assetErr := json.Marshal(copy)
		if assetErr != nil {
			return invalid
		}
		config.Xray = clean
	}
	if err = nativeValidate(config, validator); err != nil {
		invalid.Detail = validationDetail(err)
		return invalid
	}
	return ValidationResult{Valid: true, NeedsAgentValidation: needsAgent}
}

func validationDetail(err error) string {
	value := []rune(err.Error())
	if len(value) > 2048 {
		value = value[:2048]
	}
	return string(value)
}

func nodeLocalAssets(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "certificateFile" || key == "keyFile" || key == "geodata" {
				return true
			}
			if nodeLocalAssets(item) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if nodeLocalAssets(item) {
				return true
			}
		}
	case string:
		if strings.HasPrefix(value, "geoip:") || strings.HasPrefix(value, "geosite:") {
			return true
		}
	}
	return false
}
