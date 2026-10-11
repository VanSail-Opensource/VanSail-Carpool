package iplookup

import (
	"reflect"
	"testing"
)

func TestTransitProjectionExcludesUnrelatedPeerAndCustomerBranches(t *testing.T) {
	graph := buildTopology("118.127.59.0", []routeObservation{{"118.127.59.0/24", BGPPath{ASPath: []uint32{24482, 58511, 9280, 45671}}}, {"118.127.59.0/24", BGPPath{ASPath: []uint32{174, 38195, 9280, 45671}}}}, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		graph.Nodes[i].Tier1 = boolean(graph.Nodes[i].ASN == 174)
	}
	graph = CompactTopology(graph)
	for i, edge := range graph.Edges {
		graph.Edges[i].Relationship = "upstream"
		if edge.From == 24482 {
			graph.Edges[i].Relationship = "peer"
		}
	}
	graph = pruneTransitContext(graph)
	for _, node := range graph.Nodes {
		if node.ASN == 24482 {
			t.Fatal("unrelated peer appeared as a transit")
		}
	}
	for _, asn := range []uint32{9280, 58511, 38195, 174} {
		found := false
		for _, node := range graph.Nodes {
			found = found || node.ASN == asn
		}
		if !found {
			t.Fatalf("transit AS%d hidden", asn)
		}
	}
	if !reflect.DeepEqual(graph, pruneTransitContext(graph)) {
		t.Fatal("relevance projection is not idempotent")
	}
}

func TestUnknownTransitEvidenceRemainsObservedAndStopsBeyondTier1(t *testing.T) {
	graph := buildTopology("118.127.59.0", []routeObservation{{"118.127.59.0/24", BGPPath{ASPath: []uint32{24482, 174, 9280, 45671}}}}, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		graph.Nodes[i].Tier1 = boolean(graph.Nodes[i].ASN == 174)
	}
	graph = pruneTransitContext(CompactTopology(graph))
	if len(graph.Nodes) != 3 {
		t.Fatalf("non-T1 beyond a transit-free carrier retained: %+v", graph.Nodes)
	}
	if len(graph.Edges) != 2 || graph.Edges[0].Relationship != "observed" {
		t.Fatal("unknown upstream evidence discarded or asserted")
	}
}

func TestObservedPeerTransitTowardTier1IsRetained(t *testing.T) {
	graph := buildTopology("23.151.104.0", []routeObservation{{"23.151.104.0/24", BGPPath{ASPath: []uint32{2914, 40792, 44324, 53808}}}}, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		graph.Nodes[i].Tier1 = boolean(graph.Nodes[i].ASN == 2914)
	}
	graph = CompactTopology(graph)
	for i, edge := range graph.Edges {
		graph.Edges[i].Relationship = "upstream"
		if edge.From == 40792 {
			graph.Edges[i].Relationship = "peer"
		}
	}
	graph = pruneTransitContext(graph)
	if len(graph.Nodes) != 4 {
		t.Fatal("observed peer carrying the route toward Tier1 hidden")
	}
	for _, edge := range graph.Edges {
		if edge.From == 40792 && edge.Relationship != "peer" {
			t.Fatal("peer was relabeled as a provider")
		}
	}
}
