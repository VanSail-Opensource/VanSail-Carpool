package subscription

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

func templateRoot(template Template) (map[string]any, error) {
	if len(template.Document) == 0 || len(template.Document) > 256<<10 {
		return nil, errors.New("invalid template size")
	}
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(template.Document))
	decoder.UseNumber()
	if !json.Valid(template.Document) || decoder.Decode(&root) != nil || root == nil {
		return nil, errors.New("template must be a JSON object")
	}
	key := "outbounds"
	if yamlFormat(template.Format) {
		key = "proxies"
	} else if template.Format != "xray" && template.Format != "sing-box" {
		return nil, errors.New("unsupported template format")
	}
	items, ok := root[key].([]any)
	if !ok {
		return nil, fmt.Errorf("%s must contain SubscriptionProxies", key)
	}
	count := 0
	var allMarkers func(any, int) (int, error)
	allMarkers = func(value any, depth int) (int, error) {
		if depth > 64 {
			return 0, errors.New("template nesting is too deep")
		}
		total := 0
		switch item := value.(type) {
		case map[string]any:
			for key, entry := range item {
				if key == "SubscriptionProxies" {
					total++
				}
				n, err := allMarkers(entry, depth+1)
				if err != nil {
					return 0, err
				}
				total += n
			}
		case []any:
			for _, entry := range item {
				n, err := allMarkers(entry, depth+1)
				if err != nil {
					return 0, err
				}
				total += n
			}
		}
		return total, nil
	}
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			if _, exists := object["SubscriptionProxies"]; exists {
				count++
			}
		}
	}
	total, err := allMarkers(root, 0)
	if err != nil || total != 1 || count != 1 {
		return nil, errors.New("exactly one SubscriptionProxies marker is required")
	}
	return root, nil
}

func expandTemplate(value any, proxies, names []any, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("template nesting is too deep")
	}
	switch item := value.(type) {
	case []any:
		out := []any{}
		for _, entry := range item {
			if object, ok := entry.(map[string]any); ok {
				matched := false
				for key, values := range map[string][]any{"SubscriptionProxies": proxies, "SubscriptionNames": names} {
					if marker, exists := object[key]; exists {
						empty, ok := marker.(map[string]any)
						if !ok || len(empty) != 0 || len(object) != 1 {
							return nil, errors.New("insertion markers must be empty objects")
						}
						out = append(out, values...)
						matched = true
						break
					}
				}
				if matched {
					continue
				}
			}
			next, err := expandTemplate(entry, proxies, names, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, next)
		}
		return out, nil
	case map[string]any:
		for key, entry := range item {
			if key == "SubscriptionProxies" || key == "SubscriptionNames" {
				return nil, errors.New("markers are allowed only in arrays")
			}
			next, err := expandTemplate(entry, proxies, names, depth+1)
			if err != nil {
				return nil, err
			}
			item[key] = next
		}
	}
	return value, nil
}
