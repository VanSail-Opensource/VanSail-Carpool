package xnode

import (
	"context"
	"encoding/json"
	"slices"
)

// Remove only this ID, preserving explicit empty allow lists (deny everyone).
func stripEndpointGroup(options map[string]json.RawMessage, key, id string) (bool, error) {
	raw, ok := options[key]
	if !ok || string(raw) == "null" {
		return false, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return false, err
	}
	if !slices.Contains(values, id) {
		return false, nil
	}
	values = slices.DeleteFunc(values, func(value string) bool { return value == id })
	if values == nil {
		values = []string{}
	}
	options[key], _ = json.Marshal(values)
	return true, nil
}

func cleanEndpointGroup(raw json.RawMessage, id string) (json.RawMessage, bool, error) {
	var options map[string]json.RawMessage
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil, false, err
	}
	changed := false
	for _, key := range []string{"allowedGroups", "excludedGroups"} {
		updated, err := stripEndpointGroup(options, key, id)
		if err != nil {
			return nil, false, err
		}
		changed = changed || updated
	}
	if legacyRaw, ok := options["legacyHost"]; ok && string(legacyRaw) != "null" {
		var legacy map[string]json.RawMessage
		if err := json.Unmarshal(legacyRaw, &legacy); err != nil {
			return nil, false, err
		}
		updated, err := stripEndpointGroup(legacy, "excludedInternalSquads", id)
		if err != nil {
			return nil, false, err
		}
		changed = changed || updated
		if policyRaw, ok := legacy["internalSquads"]; ok && string(policyRaw) != "null" {
			var policy map[string]json.RawMessage
			if err := json.Unmarshal(policyRaw, &policy); err != nil {
				return nil, false, err
			}
			updated, err = stripEndpointGroup(policy, "squads", id)
			if err != nil {
				return nil, false, err
			}
			changed = changed || updated
			legacy["internalSquads"], _ = json.Marshal(policy)
		}
		options["legacyHost"], _ = json.Marshal(legacy)
	}
	if !changed {
		return raw, false, nil
	}
	out, err := json.Marshal(options)
	return out, true, err
}

func (s *Service) groupDeletionEndpoints(ctx context.Context, id string) ([]Endpoint, error) {
	items, err := s.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	changed := []Endpoint{}
	for _, item := range items {
		raw, updated, err := cleanEndpointGroup(item.Overrides, id)
		if err != nil {
			return nil, err
		}
		if !updated {
			continue
		}
		item.OverridesCipher, err = s.vault.Encrypt("xnode.endpoint."+item.ID, string(raw))
		if err != nil {
			return nil, err
		}
		changed = append(changed, item)
	}
	return changed, nil
}
