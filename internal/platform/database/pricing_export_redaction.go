package database

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ErrPricingExportRedaction refuses retained JSON that cannot be safely
// pseudonymized. The error deliberately contains no source value.
var ErrPricingExportRedaction = errors.New("pricing export JSON cannot be safely pseudonymized")

func pricingPseudonymScope(value string, key []byte) string {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 2 {
		return pseudonym(key, parts[0]) + ":" + parts[1]
	}
	return pseudonym(key, value)
}

func redactPricingJSON(raw string, key []byte) (string, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return "", ErrPricingExportRedaction
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", ErrPricingExportRedaction
	}
	var visit func(any) error
	visit = func(value any) error {
		switch item := value.(type) {
		case map[string]any:
			for name, child := range item {
				switch name {
				case "userId", "user_id", "actorId", "actor_id", "ownerId", "owner_id", "updatedBy", "createdBy", "approvedBy", "acknowledgedBy":
					if child == nil {
						continue
					}
					text, ok := child.(string)
					if !ok {
						return ErrPricingExportRedaction
					}
					if text != "" {
						item[name] = pseudonym(key, text)
					}
				case "userIds", "user_ids", "actorIds", "actor_ids", "ownerIds", "owner_ids":
					if child == nil {
						continue
					}
					values, ok := child.([]any)
					if !ok {
						return ErrPricingExportRedaction
					}
					for i, value := range values {
						text, ok := value.(string)
						if !ok {
							return ErrPricingExportRedaction
						}
						if text != "" {
							values[i] = pseudonym(key, text)
						}
					}
				case "scopeId", "scope_id":
					if item["scopeType"] == "user_product" || item["scope_type"] == "user_product" {
						text, ok := child.(string)
						if !ok {
							return ErrPricingExportRedaction
						}
						item[name] = pricingPseudonymScope(text, key)
					}
				default:
					if err := visit(child); err != nil {
						return err
					}
				}
			}
		case []any:
			for _, child := range item {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(value); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", ErrPricingExportRedaction
	}
	return string(encoded), nil
}
