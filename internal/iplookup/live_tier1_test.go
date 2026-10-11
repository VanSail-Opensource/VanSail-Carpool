package iplookup

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRelationshipsStopAtTier1AndReuseCliquePeerEvidence(t *testing.T) {
	queries := []string{}
	s := liveTestService(t, "caida", func(r *http.Request) (int, string) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		queries = append(queries, body["query"])
		if !strings.Contains(body["query"], `asn:"5650"`) {
			t.Error("expanded a T1 neighborhood")
		}
		return 200, `{"data":{"dataset":{"date":"2026-10-01"},"asnLinks":{"totalCount":1,"edges":[{"node":{"asn0":{"asn":"5650"},"asn1":{"asn":"174"},"relationship":"provider"}}]}}}`
	})
	graph := buildTopology("47.181.58.83", []routeObservation{{"47.181.0.0/16", BGPPath{ASPath: []uint32{701, 174, 5650}}}}, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		graph.Nodes[i].Tier1 = boolean(graph.Nodes[i].ASN != 5650)
	}
	graph.ClassificationMeta = LiveMeta{Status: "success", Source: "caida:dataset-clique", ObservedAt: "2026-10-01"}
	s.relationships(context.Background(), &graph, Config{})
	if len(queries) != 1 {
		t.Fatalf("unnecessary neighborhoods fetched: %v", queries)
	}
	for _, edge := range graph.Edges {
		if edge.From == 701 && (edge.Relationship != "peer" || edge.RelationshipMeta == nil || edge.RelationshipMeta.Source != "caida:dataset-clique") {
			t.Fatal("lost dated T1 peer context")
		}
	}
}

func TestTier1OriginNeedsNoRelationshipHTTPAndUnknownClassificationStillLooksUp(t *testing.T) {
	var calls atomic.Int32
	s := liveTestService(t, "caida", func(*http.Request) (int, string) {
		calls.Add(1)
		return 200, `{"data":{"dataset":{"date":"2026-10-01"},"asnLinks":{"totalCount":0,"edges":[]}}}`
	})
	graph := buildTopology("172.127.58.147", []routeObservation{{"172.124.0.0/14", BGPPath{ASPath: []uint32{174, 7018}}}}, LiveMeta{Status: "success"})
	for i := range graph.Nodes {
		graph.Nodes[i].Tier1 = boolean(true)
	}
	graph.ClassificationMeta = LiveMeta{Status: "success", ObservedAt: "2026-10-01"}
	s.relationships(context.Background(), &graph, Config{})
	if calls.Load() != 0 || graph.Edges[0].Relationship != "peer" {
		t.Fatal("T1-origin context unnecessarily fetched")
	}
	graph.Nodes[0].Tier1 = nil
	if got := relationshipTargets(graph); !reflect.DeepEqual(got, []uint32{174}) {
		t.Fatalf("unknown classification skipped: %v", got)
	}
}

func TestTier1NamesDoNotTriggerFallbackRequests(t *testing.T) {
	s, _, _ := integratedLiveService(t, false)
	var calls atomic.Int32
	original := s.client.Transport
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { calls.Add(1); return original.RoundTrip(r) })
	graph := BGPTopology{Origins: []uint32{7018}, Nodes: []BGPNode{{ASN: 7018, Tier1: boolean(true)}, {ASN: 174, Tier1: boolean(true)}}}
	s.rememberName(7018, "ATT-INTERNET4")
	s.resolveNames(context.Background(), &graph, Config{})
	if calls.Load() != 0 || graph.Nodes[0].Name != "ATT-INTERNET4" {
		t.Fatal("name resolution expanded transit-free context")
	}
}
