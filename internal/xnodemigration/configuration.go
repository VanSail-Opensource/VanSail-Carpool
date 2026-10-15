package xnodemigration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"slices"
)

// NativeDocument strips provider-owned account lists and selects the node's
// active inbounds. Tags remain stable when compatible and are mapped otherwise.
func NativeDocument(profileID string, document json.RawMessage, node Node) (json.RawMessage, map[string]string, error) {
	var native map[string]any
	if json.Unmarshal(document, &native) != nil {
		return nil, nil, xnode.ErrInvalid
	}
	inbounds, _ := native["inbounds"].([]any)
	active := map[string]string{}
	for _, binding := range node.Inbounds {
		tag := binding.Tag
		if tag == "" {
			return nil, nil, xnode.ErrInvalid
		}
		active[tag] = binding.UUID
	}
	tags := map[string]string{}
	renamed := map[string]string{}
	found := map[string]bool{}
	review := []string{}
	selected := []any{}
	for _, raw := range inbounds {
		inbound, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, xnode.ErrInvalid
		}
		tag, _ := inbound["tag"].(string)
		protocol, _ := inbound["protocol"].(string)
		id, enabled := active[tag]
		managed := sourceManagedProtocol(protocol)
		if !enabled && managed {
			continue
		}
		if tag == "" {
			return nil, nil, xnode.ErrInvalid
		}
		if _, duplicate := renamed[tag]; duplicate {
			return nil, nil, xnode.ErrInvalid
		}
		stable := tag
		if !validTag(stable) {
			key := id
			if !managed || key == "" {
				key = "aux:" + tag
			}
			hash := sha256.Sum256([]byte(profileID + ":" + key))
			stable = "in-" + hex.EncodeToString(hash[:12])
		}
		renamed[tag] = stable
		if enabled {
			found[tag] = true
			if managed {
				tags[id] = stable
			}
		}
		inbound["tag"] = stable
		if !managed && protocol != "dokodemo-door" && protocol != "tunnel" {
			review = append(review, stable)
		}
		if settings, _ := inbound["settings"].(map[string]any); settings != nil && managed {
			for _, key := range []string{"clients", "users", "accounts"} {
				delete(settings, key)
			}
		}
		selected = append(selected, inbound)
	}
	if len(found) != len(active) {
		return nil, nil, xnode.ErrInvalid
	}
	native["inbounds"] = selected
	if len(review) > 0 {
		// Retain unsupported utility settings for review. Native validation rejects
		// this marker until an administrator explicitly converts the listeners.
		native["MigrationReview"] = map[string]any{"unsupportedUtilityInbounds": review}
	}
	if routing, _ := native["routing"].(map[string]any); routing != nil {
		rules, _ := routing["rules"].([]any)
		for _, raw := range rules {
			rule, _ := raw.(map[string]any)
			values, _ := rule["inboundTag"].([]any)
			for i, value := range values {
				old, _ := value.(string)
				if stable := renamed[old]; stable != "" {
					values[i] = stable
				}
			}
		}
		routing["rules"] = append([]any{map[string]any{"UserRouting": map[string]any{}}}, rules...)
	} else {
		native["routing"] = map[string]any{"rules": []any{map[string]any{"UserRouting": map[string]any{}}}}
	}
	outs, _ := native["outbounds"].([]any)
	if len(outs) == 0 {
		outs = []any{map[string]any{"tag": "direct", "protocol": "freedom"}}
	}
	native["outbounds"] = append(outs, map[string]any{"UserOutbound": map[string]any{}})
	data, err := json.Marshal(native)
	return data, tags, err
}

// Remnawave's managed-client set; other listeners survive its node filtering.
// Reference: remnawave/backend@010b365ab1fabea01192b5e6ade4e98e66ee1dbd,
// src/common/helpers/xray-config/xray-config.validator.ts (leaveInbounds).
func sourceManagedProtocol(protocol string) bool {
	return protocol == "vless" || protocol == "trojan" || protocol == "shadowsocks" || protocol == "hysteria"
}
func validTag(tag string) bool {
	if len(tag) == 0 || len(tag) > 128 {
		return false
	}
	for _, r := range tag {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || slices.Contains([]rune("._-"), r)) {
			return false
		}
	}
	return true
}
func configID(node Node) string {
	hash := sha256.Sum256([]byte(node.ID + ":" + node.ProfileID))
	return "migrated-" + hex.EncodeToString(hash[:16])
}
func missingProfile(id string) error {
	return fmt.Errorf("%w: missing legacy profile %s", xnode.ErrInvalid, id)
}
