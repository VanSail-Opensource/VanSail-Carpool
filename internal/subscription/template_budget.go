package subscription

import (
	"encoding/json"
	"errors"
)

var errTemplateExpansionLimit = errors.New("rendered subscription exceeds 8 MiB")

// templateExpansionSize bounds every insertion before expanded slices are
// allocated. A small saved template may repeat SubscriptionNames many times;
// checking only the final serialized document would permit large allocations.
// Keeping the original marker bytes makes this estimate conservative.
func templateExpansionSize(root map[string]any, proxies, names []any, limit int) (int, error) {
	raw, err := json.Marshal(root)
	if err != nil {
		return 0, err
	}
	size := len(raw)
	if size > limit {
		return 0, errTemplateExpansionLimit
	}
	costs := map[string]int{}
	for key, values := range map[string][]any{"SubscriptionProxies": proxies, "SubscriptionNames": names} {
		raw, err := json.Marshal(values)
		if err != nil {
			return 0, err
		}
		costs[key] = len(raw)
	}
	var visit func(any, int) error
	visit = func(value any, depth int) error {
		if depth > 64 {
			return errors.New("template nesting is too deep")
		}
		switch item := value.(type) {
		case map[string]any:
			for key, child := range item {
				if cost := costs[key]; cost > 0 {
					if cost > limit-size {
						return errTemplateExpansionLimit
					}
					size += cost
				}
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range item {
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(root, 0); err != nil {
		return 0, err
	}
	return size, nil
}
