package database

import (
	"encoding/json"
	"fmt"
	"strings"
)

func pricingPseudonymScope(value string, key []byte) string {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 2 {
		return pseudonym(key, parts[0]) + ":" + parts[1]
	}
	return pseudonym(key, value)
}

func redactPricingJSON(raw string, key []byte) string {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return raw
	}
	var visit func(any)
	visit = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			for name, child := range item {
				switch name {
				case "userId", "user_id", "actorId", "actor_id", "updatedBy", "createdBy", "approvedBy", "acknowledgedBy":
					if text, ok := child.(string); ok && text != "" {
						item[name] = pseudonym(key, text)
					}
				case "scopeId", "scope_id":
					if item["scopeType"] == "user_product" || item["scope_type"] == "user_product" {
						item[name] = pricingPseudonymScope(fmt.Sprint(child), key)
					}
				default:
					visit(child)
				}
			}
		case []any:
			for _, child := range item {
				visit(child)
			}
		}
	}
	visit(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return string(encoded)
}
