package iplookup

// applyTier1PeerEvidence reuses CAIDA's dated clique inference for observed pairs.
// It never adds edges or expands transit-free AS neighborhoods.
func applyTier1PeerEvidence(t *BGPTopology) {
	meta := t.ClassificationMeta
	if meta.ObservedAt == "" || (meta.Status != "success" && meta.Status != "partial") {
		return
	}
	tiers := map[uint32]bool{}
	for _, node := range t.Nodes {
		tiers[node.ASN] = node.Tier1 != nil && *node.Tier1
	}
	for i, edge := range t.Edges {
		if edge.Relationship == "observed" && tiers[edge.From] && tiers[edge.To] {
			t.Edges[i].Relationship = "peer"
			provenance := LiveMeta{Status: "success", Source: "caida:dataset-clique", ObservedAt: meta.ObservedAt}
			t.Edges[i].RelationshipMeta = &provenance
			t.RelationshipMeta.ObservedAt = meta.ObservedAt
		}
	}
}
