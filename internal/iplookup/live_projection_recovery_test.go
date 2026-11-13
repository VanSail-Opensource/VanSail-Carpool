package iplookup

import (
	"math"
	"reflect"
	"testing"
)

func TestMalformedStoredRouteSummariesRemainRepairableWithoutPanicking(t *testing.T) {
	for _, tc := range []struct {
		name string
		bad  BGPRouteSummary
	}{
		{"missing hops", BGPRouteSummary{ASNs: []uint32{13335, 174}}},
		{"extra hops", BGPRouteSummary{ASNs: []uint32{13335}, Hops: []int{1}}},
		{"negative hop", BGPRouteSummary{ASNs: []uint32{13335, 174}, Hops: []int{-1}}},
		{"zero hop", BGPRouteSummary{ASNs: []uint32{13335, 174}, Hops: []int{0}}},
		{"overflow", BGPRouteSummary{ASNs: []uint32{13335, 174}, Hops: []int{math.MaxInt}}},
		{"zero ASN", BGPRouteSummary{ASNs: []uint32{0, 174}, Hops: []int{1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			good := BGPRouteSummary{ASNs: []uint32{13335, 174}, Hops: []int{1}, ObservedRoutes: 1}
			graph := BGPTopology{LiveMeta: healthyMeta(), ProjectionVersion: 2, Origins: []uint32{13335},
				Nodes: []BGPNode{{ASN: 13335}, {ASN: 174}}, Edges: []BGPEdge{{From: 174, To: 13335, Relationship: "observed"}},
				RouteSummaries: []BGPRouteSummary{tc.bad, good}}
			compact := CompactTopology(graph)
			if len(graph.RouteSummaries) != 2 || !reflect.DeepEqual(graph.RouteSummaries[0], tc.bad) {
				t.Fatal("normalization mutated the retained source")
			}
			if len(compact.RouteSummaries) != 1 || !reflect.DeepEqual(compact.RouteSummaries[0], good) || !compact.Truncated || compact.ErrorCode != "IP_DETAILS_INVALID_RESPONSE" {
				t.Fatalf("invalid projection: %+v", compact)
			}
			details := LiveDetails{Topology: compact}
			if !LivePartial(details) || !liveRetryPlanOf(details).Topology {
				t.Fatal("corrupt route evidence suppressed recovery")
			}
			// Repair consumers also tolerate a malformed historical projection
			// supplied directly by a repository implementation.
			if len(relationshipTargets(graph)) != 2 {
				t.Fatal("valid relationship targets were lost")
			}
			pruned := pruneTransitContext(graph)
			if len(pruned.RouteSummaries) != 1 || pruned.Status != "partial" {
				t.Fatalf("repair traversal lost degradation: %+v", pruned)
			}
			if !reflect.DeepEqual(compact, CompactTopology(compact)) {
				t.Fatal("normalized projection is not stable across cache reads")
			}
		})
	}
}
