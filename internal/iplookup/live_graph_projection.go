package iplookup

import (
	"encoding/json"
	"sort"
)

// cleanObservedPath reverses collector-to-origin paths and removes visual prepends.
func cleanObservedPath(path []uint32) []uint32 {
	clean := make([]uint32, 0, len(path))
	for i := len(path) - 1; i >= 0; i-- {
		if len(clean) == 0 || clean[len(clean)-1] != path[i] {
			clean = append(clean, path[i])
		}
	}
	return clean
}

func projectObservedTopology(t BGPTopology) BGPTopology {
	visible, tiers := map[uint32]bool{}, map[uint32]bool{}
	counts, pairs := map[uint32]int{}, map[[2]uint32]int{}
	for _, asn := range t.Origins {
		visible[asn] = true
	}
	for _, node := range t.Nodes {
		tiers[node.ASN] = node.Tier1 != nil && *node.Tier1
	}
	paths := make([][]uint32, 0, len(t.Paths))
	for _, observation := range t.Paths {
		path := cleanObservedPath(observation.ASPath)
		paths = append(paths, path)
		seen, seenPairs := map[uint32]bool{}, map[[2]uint32]bool{}
		for i, asn := range path {
			if i == 0 || i <= 3 && !tiers[path[0]] || tiers[asn] {
				visible[asn] = true
			}
			if !seen[asn] {
				counts[asn]++
				seen[asn] = true
			}
			if i > 0 {
				pair := [2]uint32{asn, path[i-1]}
				if !seenPairs[pair] {
					pairs[pair]++
					seenPairs[pair] = true
				}
			}
		}
	}
	nodes, edges := []BGPNode{}, []BGPEdge{}
	for _, node := range t.Nodes {
		if visible[node.ASN] {
			count := counts[node.ASN]
			node.ObservedRoutes = &count
			nodes = append(nodes, node)
		}
	}
	for _, edge := range t.Edges {
		if edge.From != edge.To && visible[edge.From] && visible[edge.To] {
			count := pairs[[2]uint32{edge.From, edge.To}]
			edge.ObservedRoutes = &count
			edges = append(edges, edge)
		}
	}
	summaries := map[string]BGPRouteSummary{}
	for _, path := range paths {
		route := BGPRouteSummary{ASNs: []uint32{}, Hops: []int{}}
		previous := -1
		for i, asn := range path {
			if !visible[asn] || tiers[path[0]] && !tiers[asn] && i > 0 {
				continue
			}
			if previous >= 0 {
				route.Hops = append(route.Hops, i-previous)
			}
			route.ASNs = append(route.ASNs, asn)
			previous = i
		}
		if len(route.ASNs) == 0 {
			continue
		}
		key, _ := json.Marshal(struct {
			ASNs []uint32
			Hops []int
		}{route.ASNs, route.Hops})
		old := summaries[string(key)]
		route.ObservedRoutes = old.ObservedRoutes + 1
		summaries[string(key)] = route
	}
	t.Nodes, t.Edges, t.Paths, t.Tier1Links = nodes, edges, []BGPPath{}, []Tier1Link{}
	t.ProjectionVersion, t.RouteSummaries = 2, []BGPRouteSummary{}
	keys := make([]string, 0, len(summaries))
	for key := range summaries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t.RouteSummaries = append(t.RouteSummaries, summaries[key])
	}
	return t
}

// relationshipTargets keeps closest, most observed ASNs ahead of distant carriers.
func relationshipTargets(t BGPTopology) []uint32 {
	if t.ProjectionVersion != 2 {
		return append([]uint32{}, t.Origins...)
	}
	distance := map[uint32]int{}
	for _, route := range t.RouteSummaries {
		hops := 0
		for i, asn := range route.ASNs {
			if i > 0 {
				hops += route.Hops[i-1]
			}
			if previous, ok := distance[asn]; !ok || hops < previous {
				distance[asn] = hops
			}
		}
	}
	nodes := append([]BGPNode{}, t.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if distance[a.ASN] != distance[b.ASN] {
			return distance[a.ASN] < distance[b.ASN]
		}
		count := func(n BGPNode) int {
			if n.ObservedRoutes != nil {
				return *n.ObservedRoutes
			}
			return 0
		}
		if count(a) != count(b) {
			return count(a) > count(b)
		}
		return a.ASN < b.ASN
	})
	ids := make([]uint32, 0, min(32, len(nodes)))
	for _, node := range nodes[:min(32, len(nodes))] {
		ids = append(ids, node.ASN)
	}
	return ids
}
