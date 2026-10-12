package iplookup

import (
	"context"
	"sync"
	"testing"
	"time"
)

type snapshotMemory struct {
	mu   sync.Mutex
	rows map[string]LiveDetails
}

func (m *snapshotMemory) CachedIPDetails(_ context.Context, ip string) (*LiveDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[ip]
	if !ok {
		return nil, nil
	}
	r.CacheMatch = "exact"
	return &r, nil
}
func (m *snapshotMemory) SaveIPDetails(_ context.Context, _ string, r LiveDetails, _ time.Time) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows == nil {
		m.rows = map[string]LiveDetails{}
	}
	r.SnapshotID = r.IP
	m.rows[r.IP] = r
	return r.IP, nil
}

func TestCompactTopologyRetainsAllObservedT1sWithoutOriginalPaths(t *testing.T) {
	t.Parallel()
	graph := BGPTopology{Origins: []uint32{10, 11}, Nodes: []BGPNode{{ASN: 10}, {ASN: 11}, {ASN: 20}, {ASN: 21}, {ASN: 30}, {ASN: 174, Tier1: boolean(true)}, {ASN: 701, Tier1: boolean(true)}},
		Edges: []BGPEdge{{From: 20, To: 10, Relationship: "upstream"}, {From: 11, To: 21, Relationship: "customer"}, {From: 30, To: 20}},
		Paths: []BGPPath{{ASPath: []uint32{701, 174, 30, 20, 20, 10, 10}}, {ASPath: []uint32{174, 21, 11}}}}
	compact := CompactTopology(graph)
	if len(compact.Paths) != 0 || len(compact.Nodes) != 7 || len(compact.Edges) != 3 || len(compact.RouteSummaries) != 2 || compact.ProjectionVersion != 2 {
		t.Fatalf("invalid compact projection: %+v", compact)
	}
	if len(graph.Paths) != 2 || len(graph.Nodes) != 7 {
		t.Fatal("mutated transient source")
	}
	if len(CompactTopology(compact).RouteSummaries) != 2 {
		t.Fatal("compaction not idempotent")
	}
}

func TestLivePartialIgnoresCoverageLimitedRelationshipLineage(t *testing.T) {
	healthy := LiveDetails{Topology: BGPTopology{LiveMeta: LiveMeta{Status: "success"}, RelationshipMeta: LiveMeta{Status: "partial", Source: "caida,cloudflare"}, ClassificationMeta: LiveMeta{Status: "success"}}}
	if LivePartial(healthy) {
		t.Fatal("coverage-limited relationship lineage marked the snapshot partial")
	}
	failing := healthy
	failing.Topology.RelationshipMeta = LiveMeta{Status: "partial", ErrorCode: "IP_DETAILS_INVALID_RESPONSE"}
	if !LivePartial(failing) {
		t.Fatal("relationship fetch failure lost its partial marker")
	}
	errored := healthy
	errored.Topology.RelationshipMeta = LiveMeta{Status: "error", ErrorCode: "IP_DETAILS_UNAVAILABLE"}
	if !LivePartial(errored) {
		t.Fatal("relationship error lost its partial marker")
	}
}
