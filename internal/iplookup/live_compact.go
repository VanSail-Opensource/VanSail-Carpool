package iplookup

import "sort"

// CompactTopology retains only origins, observed direct neighbors and dated T1s.
// Original paths are used transiently to summarize reachability, then discarded.
func CompactTopology(t BGPTopology) BGPTopology {
	origins, visible, tiers := map[uint32]bool{}, map[uint32]bool{}, map[uint32]bool{}
	for _, asn := range t.Origins {
		origins[asn], visible[asn] = true, true
	}
	for _, node := range t.Nodes {
		if node.Tier1 != nil && *node.Tier1 {
			tiers[node.ASN], visible[node.ASN] = true, true
		}
	}
	edges := []BGPEdge{}
	for _, edge := range t.Edges {
		if edge.From == edge.To {
			continue
		}
		if origins[edge.From] || origins[edge.To] {
			visible[edge.From], visible[edge.To] = true, true
			edges = append(edges, edge)
		}
	}
	links := map[[3]uint32]Tier1Link{}
	for _, link := range t.Tier1Links {
		links[[3]uint32{link.Origin, link.Neighbor, link.Tier1}] = link
	}
	for _, path := range t.Paths {
		clean := []uint32{}
		for _, asn := range path.ASPath {
			if len(clean) == 0 || clean[len(clean)-1] != asn {
				clean = append(clean, asn)
			}
		}
		if len(clean) < 3 {
			continue
		}
		origin, neighbor := clean[len(clean)-1], clean[len(clean)-2]
		if !origins[origin] {
			continue
		}
		for i, asn := range clean[:len(clean)-2] {
			if !tiers[asn] || asn == origin || asn == neighbor {
				continue
			}
			key := [3]uint32{origin, neighbor, asn}
			hops := len(clean) - 2 - i
			if previous, ok := links[key]; !ok || hops < previous.Hops {
				links[key] = Tier1Link{origin, neighbor, asn, hops}
			}
		}
	}
	nodes := []BGPNode{}
	for _, node := range t.Nodes {
		if visible[node.ASN] {
			nodes = append(nodes, node)
		}
	}
	t.Nodes, t.Edges, t.Paths, t.Tier1Links = nodes, edges, []BGPPath{}, []Tier1Link{}
	for _, link := range links {
		t.Tier1Links = append(t.Tier1Links, link)
	}
	sort.Slice(t.Tier1Links, func(i, j int) bool {
		a, b := t.Tier1Links[i], t.Tier1Links[j]
		if a.Origin != b.Origin {
			return a.Origin < b.Origin
		}
		if a.Neighbor != b.Neighbor {
			return a.Neighbor < b.Neighbor
		}
		return a.Tier1 < b.Tier1
	})
	return t
}

// CompactLiveDetails is the single public and persistence projection boundary.
func CompactLiveDetails(r LiveDetails) LiveDetails {
	r.Topology = CompactTopology(r.Topology)
	if r.SourceIP == "" {
		r.SourceIP = r.IP
	}
	if r.CacheMatch == "" {
		r.CacheMatch = "none"
	}
	return r
}
