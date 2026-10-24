package subscription

import (
	"errors"
	"fmt"
)

// templateProxyNames allocates names after client mapping, retaining every
// credential-bearing endpoint. Only colliding export labels receive a stable
// public proxy-ID suffix; inventory labels and private billing IDs stay intact.
func templateProxyNames(format string, root map[string]any, proxies []Proxy, values []any) ([]any, error) {
	key, tag := "outbounds", "tag"
	used := map[string]bool{}
	if yamlFormat(format) {
		key, tag = "proxies", "name"
		for _, name := range []string{"DIRECT", "REJECT", "REJECT-DROP", "PASS", "GLOBAL"} {
			used[name] = true
		}
	}
	for _, key := range []string{key, "proxy-groups"} {
		items, _ := root[key].([]any)
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			if name, ok := item[tag].(string); ok {
				used[name] = true
			}
		}
	}
	counts := map[string]int{}
	for _, raw := range values {
		item := raw.(map[string]any)
		name, ok := item[tag].(string)
		if !ok || name == "" {
			return nil, errors.New("mapped proxy name is required")
		}
		counts[name]++
	}
	names := make([]any, 0, len(values))
	for index, raw := range values {
		item := raw.(map[string]any)
		name := item[tag].(string)
		if used[name] || counts[name] > 1 {
			base := fmt.Sprintf("%s [%s]", name, proxies[index].Tag()[6:14])
			name = base
			for n := 2; used[name] || counts[name] > 0; n++ {
				name = fmt.Sprintf("%s-%d", base, n)
			}
		}
		item[tag] = name
		used[name] = true
		names = append(names, name)
	}
	return names, nil
}
