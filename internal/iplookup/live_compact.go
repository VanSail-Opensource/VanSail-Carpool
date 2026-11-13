package iplookup

import "sort"

// CompactTopology preserves the versioned projection on repeated reads.
func CompactTopology(t BGPTopology) BGPTopology {
	if len(t.Paths) > 0 {
		return projectObservedTopology(t)
	}
	if t.ProjectionVersion == 2 {
		if t.Tier1Links == nil {
			t.Tier1Links = []Tier1Link{}
		}
		return validateRouteSummaries(t)
	}
	return compactLegacyTopology(t)
}

// Stored versioned projections can outlive their writer. Reject malformed hop
// vectors before public display or repair traversals, retaining valid evidence.
func validateRouteSummaries(t BGPTopology) BGPTopology {
	var valid []BGPRouteSummary
	for i, route := range t.RouteSummaries {
		if validRouteSummary(route) {
			if valid != nil {
				valid = append(valid, route)
			}
			continue
		}
		if valid == nil {
			valid = append(make([]BGPRouteSummary, 0, len(t.RouteSummaries)), t.RouteSummaries[:i]...)
		}
		if t.Status != "error" {
			t.Status = "partial"
		}
		if t.ErrorCode == "" {
			t.ErrorCode = "IP_DETAILS_INVALID_RESPONSE"
		}
		t.Truncated = true
	}
	if valid != nil {
		t.RouteSummaries = valid
	}
	return t
}

func validRouteSummary(route BGPRouteSummary) bool {
	if len(route.ASNs) == 0 || len(route.ASNs) > 128 || len(route.Hops) != len(route.ASNs)-1 {
		return false
	}
	distance := 0
	for i, asn := range route.ASNs {
		if asn == 0 {
			return false
		}
		if i > 0 {
			hops := route.Hops[i-1]
			if hops < 1 || hops > 127-distance {
				return false
			}
			distance += hops
		}
	}
	return true
}

// compactLegacyTopology never upgrades snapshots whose original evidence is gone.
func compactLegacyTopology(t BGPTopology) BGPTopology {
	origins, visible := map[uint32]bool{}, map[uint32]bool{}
	for _, asn := range t.Origins {
		origins[asn], visible[asn] = true, true
	}
	for _, node := range t.Nodes {
		if node.Tier1 != nil && *node.Tier1 {
			visible[node.ASN] = true
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
	if r.Shodan.CPEs == nil {
		r.Shodan.CPEs = []string{}
	}
	if r.Shodan.Tags == nil {
		r.Shodan.Tags = []string{}
	}
	if r.Shodan.Vulns == nil {
		r.Shodan.Vulns = []string{}
	}
	if r.SourceIP == "" {
		r.SourceIP = r.IP
	}
	if r.CacheMatch == "" {
		r.CacheMatch = "none"
	}
	return r
}
