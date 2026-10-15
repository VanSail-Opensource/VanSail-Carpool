package xnode

import (
	"encoding/json"
	"slices"
)

// Host ACLs stay inside the encrypted endpoint options. Reading the historical
// host also protects endpoints imported before allow-only support was added.
func filterEndpointGroups(groups []string, raw json.RawMessage) ([]string, error) {
	var options struct {
		Excluded []string  `json:"excludedGroups"`
		Allowed  *[]string `json:"allowedGroups"`
		Legacy   struct {
			Excluded []string `json:"excludedInternalSquads"`
			Policy   *struct {
				Mode   string   `json:"mode"`
				Squads []string `json:"squads"`
			} `json:"internalSquads"`
		} `json:"legacyHost"`
	}
	if json.Unmarshal(raw, &options) != nil {
		return nil, ErrInvalid
	}
	policy := options.Legacy.Policy
	if policy != nil && policy.Mode != "EXCLUDE" && policy.Mode != "ALLOW_ONLY" {
		return nil, ErrInvalid
	}
	result := []string{}
	for _, group := range groups {
		if slices.Contains(options.Excluded, group) || slices.Contains(options.Legacy.Excluded, group) {
			continue
		}
		if options.Allowed != nil && !slices.Contains(*options.Allowed, group) {
			continue
		}
		if policy != nil && (slices.Contains(policy.Squads, group) != (policy.Mode == "ALLOW_ONLY")) {
			continue
		}
		result = append(result, group)
	}
	return result, nil
}
