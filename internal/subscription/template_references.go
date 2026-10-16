package subscription

import (
	"errors"
	"strings"
)

func validateSelectorCycles(groups []any, tag, refs string) error {
	edges := map[string][]string{}
	for _, raw := range groups {
		group, _ := raw.(map[string]any)
		name, _ := group[tag].(string)
		items, _ := group[refs].([]any)
		for _, item := range items {
			if next, ok := item.(string); ok {
				edges[name] = append(edges[name], next)
			}
		}
	}
	state := map[string]int{}
	var visit func(string, int) error
	visit = func(name string, depth int) error {
		if state[name] == 1 || depth > 64 {
			return errors.New("cyclic or excessively deep selector references")
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		for _, next := range edges[name] {
			if err := visit(next, depth+1); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}
	for name := range edges {
		if err := visit(name, 0); err != nil {
			return err
		}
	}
	return nil
}

func validateClashRules(root map[string]any, known map[string]bool) error {
	raw, exists := root["rules"]
	if !exists {
		return nil
	}
	rules, ok := raw.([]any)
	if !ok {
		return errors.New("rules must be an array")
	}
	for _, raw := range rules {
		rule, ok := raw.(string)
		if !ok {
			return errors.New("invalid Clash rule")
		}
		parts := strings.Split(rule, ",")
		if len(parts) < 2 {
			return errors.New("invalid Clash rule")
		}
		target := strings.TrimSpace(parts[len(parts)-1])
		if target == "no-resolve" && len(parts) > 2 {
			target = strings.TrimSpace(parts[len(parts)-2])
		}
		if !known[target] {
			return errors.New("rule references an unknown proxy or group")
		}
	}
	return nil
}
