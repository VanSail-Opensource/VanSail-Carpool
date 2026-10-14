package xnode

func expandMarker(objects []any, key string, injected []any) ([]any, bool, error) {
	out := []any{}
	found := false
	for _, raw := range objects {
		object, ok := raw.(map[string]any)
		if !ok {
			return nil, false, ErrInvalid
		}
		if marker, exists := object[key]; exists {
			empty, ok := marker.(map[string]any)
			if !ok || len(empty) != 0 || len(object) != 1 || found {
				return nil, false, ErrInvalid
			}
			if key == "UserOutbound" && len(out) == 0 {
				return nil, false, ErrInvalid
			}
			found = true
			out = append(out, injected...)
		} else {
			out = append(out, raw)
		}
	}
	return out, found, nil
}
func appendSharedSnippet(native map[string]any, kind string, objects []any) error {
	key, marker := "outbounds", "UserOutbound"
	parent := native
	if kind == "routing" {
		marker, key = "UserRouting", "rules"
		parent, _ = native["routing"].(map[string]any)
		if parent == nil {
			parent = map[string]any{}
			native["routing"] = parent
		}
	} else if kind != "outbound" {
		return ErrInvalid
	}
	current, _ := parent[key].([]any)
	if kind == "routing" {
		parent[key] = append(current, objects...)
		return nil
	}
	out := []any{}
	added := false
	for _, raw := range current {
		object, _ := raw.(map[string]any)
		if _, ok := object[marker]; ok && !added {
			out = append(out, objects...)
			added = true
		}
		out = append(out, raw)
	}
	if !added {
		out = append(out, objects...)
	}
	parent[key] = out
	return nil
}
