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
	selected := []any{}
	for _, raw := range inbounds {
		inbound, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, xnode.ErrInvalid
		}
		tag, _ := inbound["tag"].(string)
		id, enabled := active[tag]
		if !enabled {
			continue
		}
		stable := tag
		if !validTag(stable) {
			hash := sha256.Sum256([]byte(profileID + ":" + id))
			stable = "in-" + hex.EncodeToString(hash[:12])
		}
		tags[id] = stable
		inbound["tag"] = stable
		if settings, _ := inbound["settings"].(map[string]any); settings != nil {
			for _, key := range []string{"clients", "users", "accounts"} {
				delete(settings, key)
			}
		}
		selected = append(selected, inbound)
	}
	if len(selected) != len(active) {
		return nil, nil, xnode.ErrInvalid
	}
	native["inbounds"] = selected
	if routing, _ := native["routing"].(map[string]any); routing != nil {
		rules, _ := routing["rules"].([]any)
		for _, raw := range rules {
			rule, _ := raw.(map[string]any)
			values, _ := rule["inboundTag"].([]any)
			for i, value := range values {
				old, _ := value.(string)
				if id := active[old]; id != "" {
					values[i] = tags[id]
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
