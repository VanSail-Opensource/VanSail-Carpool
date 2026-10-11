package iplookup

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestObservedProjectionRetainsThreeHopsCountsAndCompressedEvidence(t *testing.T) {
	rows := []routeObservation{
		{"118.127.59.0/24", BGPPath{ASPath: []uint32{701, 888, 999, 174, 38195, 9280, 45671, 45671}}},
		{"118.127.59.0/24", BGPPath{ASPath: []uint32{3257, 38195, 9280, 45671}}},
		{"118.127.59.0/24", BGPPath{ASPath: []uint32{24482, 58511, 9280, 45671}}},
	}
	graph := buildTopology("118.127.59.0", rows, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		graph.Nodes[i].Tier1 = boolean(graph.Nodes[i].ASN == 701)
	}
	compact := CompactTopology(graph)
	counts := map[uint32]int{}
	for _, node := range compact.Nodes {
		counts[node.ASN] = *node.ObservedRoutes
	}
	if counts[45671] != 3 || counts[9280] != 3 || counts[38195] != 2 || counts[58511] != 1 {
		t.Fatalf("wrong observation counts: %v", counts)
	}
	if _, ok := counts[999]; ok {
		t.Fatal("retained an omitted fourth hop")
	}
	found := false
	for _, route := range compact.RouteSummaries {
		if reflect.DeepEqual(route.ASNs, []uint32{45671, 9280, 38195, 174, 701}) {
			found = reflect.DeepEqual(route.Hops, []int{1, 1, 1, 3})
		}
	}
	if !found {
		t.Fatalf("missing compressed observed segment: %+v", compact.RouteSummaries)
	}
	if !reflect.DeepEqual(compact, CompactTopology(compact)) {
		t.Fatal("projection changed on a cache read")
	}
	if len(graph.Paths) != 3 || len(compact.Paths) != 0 {
		t.Fatal("original paths mutated or persisted")
	}
}

func TestThreeHopRelationshipsClassifyIntermediateObservedTransitPairs(t *testing.T) {
	s := liveTestService(t, "caida", func(r *http.Request) (int, string) {
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		return 200, `{"data":{"dataset":{"date":"2026-10-01"},"asnLinks":{"totalCount":3,"edges":[{"node":{"asn0":{"asn":"45671"},"asn1":{"asn":"9280"},"relationship":"provider"}},{"node":{"asn0":{"asn":"9280"},"asn1":{"asn":"38195"},"relationship":"provider"}},{"node":{"asn0":{"asn":"174"},"asn1":{"asn":"38195"},"relationship":"customer"}}]}}}`
	})
	graph := buildTopology("118.127.59.0", []routeObservation{{"118.127.59.0/24", BGPPath{ASPath: []uint32{174, 38195, 9280, 45671}}}}, LiveMeta{Status: "success"})
	s.relationships(context.Background(), &graph, Config{})
	if len(graph.Edges) != 3 {
		t.Fatal("lost intermediate edges")
	}
	for _, edge := range graph.Edges {
		if edge.Relationship != "upstream" || edge.RelationshipMeta == nil || edge.RelationshipMeta.ObservedAt != "2026-10-01" {
			t.Fatalf("wrong direction/provenance: %+v", edge)
		}
	}
}

func TestTier1OriginRetainsOnlyObservedTier1Context(t *testing.T) {
	graph := buildTopology("172.127.58.147", []routeObservation{{"172.124.0.0/14", BGPPath{ASPath: []uint32{24482, 2914, 7018}}}, {"172.124.0.0/14", BGPPath{ASPath: []uint32{8218, 6461, 7018}}}}, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		id := graph.Nodes[i].ASN
		graph.Nodes[i].Tier1 = boolean(id == 7018 || id == 2914 || id == 6461)
	}
	compact := CompactTopology(graph)
	if len(compact.Nodes) != 3 {
		t.Fatalf("non-T1 collector leaked into transit-free graph: %+v", compact.Nodes)
	}
	for _, node := range compact.Nodes {
		if node.Tier1 == nil || !*node.Tier1 {
			t.Fatal("retained a non-T1 node")
		}
	}
}

func TestNonTier1OriginRetainsAllObservedNearTransits(t *testing.T) {
	rows := []routeObservation{{"23.151.104.0/24", BGPPath{ASPath: []uint32{3356, 216211, 44324, 53808}}}, {"23.151.104.0/24", BGPPath{ASPath: []uint32{1299, 2497, 216211, 44324, 53808}}}, {"23.151.104.0/24", BGPPath{ASPath: []uint32{2914, 40792, 44324, 53808}}}}
	graph := buildTopology("23.151.104.0", rows, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		id := graph.Nodes[i].ASN
		graph.Nodes[i].Tier1 = boolean(id == 3356 || id == 1299 || id == 2914)
	}
	compact := CompactTopology(graph)
	if len(compact.Nodes) != 8 {
		t.Fatalf("lost a transit: %+v", compact.Nodes)
	}
	for _, asn := range []uint32{44324, 216211, 2497, 40792} {
		found := false
		for _, node := range compact.Nodes {
			found = found || node.ASN == asn
		}
		if !found {
			t.Fatalf("transit AS%d hidden", asn)
		}
	}
}

func TestEmptyVersionedTopologyKeepsArrayContract(t *testing.T) {
	graph := CompactTopology(buildTopology("1.1.1.1", nil, LiveMeta{Status: "error"}))
	raw, err := json.Marshal(graph)
	if err != nil || strings.Contains(string(raw), `"tier1Links":null`) || len(graph.Paths) != 0 || graph.ProjectionVersion != 2 {
		t.Fatalf("invalid empty topology: %s %v", raw, err)
	}
}
