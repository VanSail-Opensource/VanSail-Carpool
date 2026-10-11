package iplookup

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBulkOriginRelationsPaginateReverseAndDoNotAddEdges(t *testing.T) {
	queries := []string{}
	s := liveTestService(t, "caida", func(r *http.Request) (int, string) {
		var input map[string]string
		_ = json.NewDecoder(r.Body).Decode(&input)
		queries = append(queries, input["query"])
		if strings.Contains(input["query"], "offset:0") {
			return 200, `{"data":{"dataset":{"date":"2026-10-01"},"asnLinks":{"totalCount":2,"edges":[{"node":{"asn0":{"asn":"5650"},"asn1":{"asn":"174"},"relationship":"provider"}}]}}}`
		}
		return 200, `{"data":{"dataset":{"date":"2026-10-01"},"asnLinks":{"totalCount":2,"edges":[{"node":{"asn0":{"asn":"701"},"asn1":{"asn":"5650"},"relationship":"customer"}},{"node":{"asn0":{"asn":"5650"},"asn1":{"asn":"3356"},"relationship":"provider"}}]}}}`
	})
	graph := BGPTopology{Origins: []uint32{5650}, Edges: []BGPEdge{{From: 174, To: 5650, Relationship: "observed"}, {From: 701, To: 5650, Relationship: "observed"}, {From: 9002, To: 174, Relationship: "observed"}}}
	s.relationships(context.Background(), &graph, Config{})
	if len(graph.Edges) != 3 || graph.Edges[0].Relationship != "upstream" || graph.Edges[1].Relationship != "upstream" || graph.Edges[2].Relationship != "observed" {
		t.Fatalf("incorrect observed edge annotations: %+v", graph.Edges)
	}
	if len(queries) != 2 || !strings.Contains(queries[1], "offset:1") {
		t.Fatal("pagination skipped unresolved pairs")
	}
	if graph.RelationshipMeta.Status != "partial" || graph.RelationshipMeta.ObservedAt != "2026-10-01" {
		t.Fatal("unresolved indirect observations lost their status/date")
	}
}

func TestBulkRelationsPreserveKnownDataFromPartialGraphQLResponse(t *testing.T) {
	s := liveTestService(t, "caida", func(*http.Request) (int, string) {
		return 200, `{"data":{"dataset":{"date":"2026-10-01"},"asnLinks":{"totalCount":1,"edges":[{"node":{"asn0":{"asn":"7018"},"asn1":{"asn":"174"},"relationship":"peer"}}]}},"errors":[{"message":"another link unavailable"}]}`
	})
	graph := BGPTopology{Origins: []uint32{7018}, Edges: []BGPEdge{{From: 174, To: 7018, Relationship: "observed"}}}
	s.relationships(context.Background(), &graph, Config{})
	if graph.Edges[0].Relationship != "peer" || graph.RelationshipMeta.Status != "partial" {
		t.Fatal("a partial response discarded the known direct relationship")
	}
}

func TestAbsentDatasetCliqueKeepsEveryClassificationUnknown(t *testing.T) {
	for _, clique := range []string{`null`, `[]`, `["invalid"]`} {
		t.Run(clique, func(t *testing.T) {
			s := liveTestService(t, "caida", func(*http.Request) (int, string) {
				return 200, `{"data":{"dataset":{"date":"2026-10-01","clique":` + clique + `}}}`
			})
			graph := BGPTopology{Nodes: []BGPNode{{ASN: 13335, Tier1: boolean(true)}}}
			s.classify(context.Background(), &graph)
			if graph.Nodes[0].Tier1 != nil || graph.ClassificationMeta.Status != "error" {
				t.Fatal("invalid clique created an asserted classification")
			}
		})
	}
}

func TestKnownDirectPairsSkipUnnecessaryCloudflareFallback(t *testing.T) {
	s, _, _ := integratedLiveService(t, false)
	original := s.client.Transport
	var fallback atomic.Int32
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/rel") {
			fallback.Add(1)
		}
		return original.RoundTrip(r)
	})
	graph := BGPTopology{Origins: []uint32{13335}, Edges: []BGPEdge{{From: 174, To: 13335, Relationship: "observed"}, {From: 12345, To: 174, Relationship: "observed"}}}
	s.relationships(context.Background(), &graph, Config{})
	if fallback.Load() != 0 || graph.Edges[0].Relationship != "upstream" || graph.RelationshipMeta.ObservedAt != "2026-10-01" {
		t.Fatal("indirect unknown edges triggered redundant fallback or lost direct provenance")
	}
}
