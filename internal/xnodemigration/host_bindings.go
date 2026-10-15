package xnodemigration

import (
	"slices"
	"sort"
)

// An empty source node relation does not remove the subscription host. Resolve
// it through the host's inbound and the nodes that actually run that inbound.
func hostBindingNodes(explicit []string, inbound string, bindings map[string]map[string]string) []string {
	nodes := []string{}
	for node, tags := range bindings {
		if tags[inbound] != "" && (len(explicit) == 0 || slices.Contains(explicit, node)) {
			nodes = append(nodes, node)
		}
	}
	sort.Strings(nodes)
	return nodes
}
