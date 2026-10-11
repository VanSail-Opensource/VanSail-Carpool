package iplookup

import (
	"encoding/json"
	"sort"
)

// pruneTransitContext excludes classified non-transit branches and ASNs beyond T1s.
// A peer can carry a customer's route toward T1; its link type stays explicit.
func pruneTransitContext(t BGPTopology) BGPTopology {
	if t.ProjectionVersion != 2 {
		return t
	}
	tiers, keep := map[uint32]bool{}, map[uint32]bool{}
	classified := true
	for _, node := range t.Nodes {
		tiers[node.ASN] = node.Tier1 != nil && *node.Tier1
		classified = classified && node.Tier1 != nil
	}
	for _, origin := range t.Origins {
		keep[origin] = true
	}
	for _, route := range t.RouteSummaries {
		if len(route.ASNs) == 0 {
			continue
		}
		transit := !tiers[route.ASNs[0]]
		firstTier := -1
		for i, asn := range route.ASNs {
			if tiers[asn] {
				firstTier = i
				break
			}
		}
		distance := 0
		for i, asn := range route.ASNs {
			if i == 0 {
				continue
			}
			distance += route.Hops[i-1]
			if transit {
				if route.Hops[i-1] > 1 {
					transit = false
				} else {
					relation := observedRelation(t.Edges, route.ASNs[i-1], asn)
					transit = relation == "upstream" || relation == "observed" && (!classified || i == 1)
				}
			}
			if tiers[asn] {
				keep[asn] = true
				transit = false
			} else if distance <= 3 && (transit || firstTier > i) {
				keep[asn] = true
			}
		}
	}
	nodes, edges := []BGPNode{}, []BGPEdge{}
	for _, node := range t.Nodes {
		if keep[node.ASN] {
			nodes = append(nodes, node)
		}
	}
	for _, edge := range t.Edges {
		if keep[edge.From] && keep[edge.To] {
			edges = append(edges, edge)
		}
	}
	summaries := map[string]BGPRouteSummary{}
	for _, original := range t.RouteSummaries {
		route := BGPRouteSummary{ASNs: []uint32{}, Hops: []int{}}
		hops := 0
		for i, asn := range original.ASNs {
			if i > 0 {
				hops += original.Hops[i-1]
			}
			if !keep[asn] {
				continue
			}
			if len(route.ASNs) > 0 {
				route.Hops = append(route.Hops, hops)
			}
			route.ASNs = append(route.ASNs, asn)
			hops = 0
		}
		if len(route.ASNs) == 0 {
			continue
		}
		key, _ := json.Marshal(struct {
			ASNs []uint32
			Hops []int
		}{route.ASNs, route.Hops})
		route.ObservedRoutes = original.ObservedRoutes + summaries[string(key)].ObservedRoutes
		summaries[string(key)] = route
	}
	keys := make([]string, 0, len(summaries))
	for key := range summaries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	t.Nodes, t.Edges, t.RouteSummaries = nodes, edges, []BGPRouteSummary{}
	for _, key := range keys {
		t.RouteSummaries = append(t.RouteSummaries, summaries[key])
	}
	return t
}

// Edges describe From relative to To, regardless of original path orientation.
func observedRelation(edges []BGPEdge, near, far uint32) string {
	for _, edge := range edges {
		if edge.To == near && edge.From == far {
			return edge.Relationship
		}
		if edge.From == near && edge.To == far {
			return reverseRelationship(edge.Relationship)
		}
	}
	return "observed"
}
