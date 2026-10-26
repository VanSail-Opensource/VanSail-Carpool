package subscription

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// templateProxyNames allocates names after client mapping, retaining every
// credential-bearing endpoint. Collisions receive numeric labels in public-ID
// order; inventory labels and private billing IDs stay intact.
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
	order := make([]int, len(values))
	for index := range order {
		order[index] = index
	}
	slices.SortStableFunc(order, func(a, b int) int { return strings.Compare(proxies[a].ID, proxies[b].ID) })
	names := make([]any, len(values))
	for _, index := range order {
		item := values[index].(map[string]any)
		name := item[tag].(string)
		if used[name] {
			base := name
			for n := 2; ; n++ {
				name = fmt.Sprintf("%s (%d)", base, n)
				if !used[name] && counts[name] == 0 {
					break
				}
			}
		}
		item[tag] = name
		used[name] = true
		names[index] = name
	}
	return names, nil
}
