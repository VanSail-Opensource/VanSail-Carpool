package xnode

import (
	"errors"
	"slices"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
)

func TestRoutingEndpointResolutionPreservesDefaultsAndRejectsOwnershipMismatch(t *testing.T) {
	a := Endpoint{ID: "a-first", NodeID: "a", InboundTag: "primary"}
	a2 := Endpoint{ID: "a-second", NodeID: "a", InboundTag: "alternate"}
	b := Endpoint{ID: "b-first", NodeID: "b", InboundTag: "primary"}
	for _, test := range []struct {
		name, source, target string
		valid, legacy        bool
	}{
		{"legacy", "", "", true, true},
		{"specific source", a2.ID, b.ID, true, false},
		{"wrong node", b.ID, b.ID, false, false},
		{"not authorized", "private-host", b.ID, false, false},
		{"unavailable target", a.ID, "removed-host", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			in := routingInput{endpoints: map[string]Endpoint{"a": a, "b": b}, byEndpoint: map[string]Endpoint{a.ID: a, a2.ID: a2, b.ID: b}, sources: map[string]Endpoint{}, targets: map[string]Endpoint{}}
			choice := RoutingChoice{ID: "rule", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "google", InboundEndpointID: test.source, OutboundEndpointID: test.target}
			err := in.resolveChoices([]RoutingChoice{choice})
			if !test.valid {
				var diagnostic *routingmatch.Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != "routing_endpoint_unavailable" {
					t.Fatal("endpoint escaped ownership/node correspondence", err)
				}
				return
			}
			if err != nil || in.legacy != test.legacy || len(in.choices) != 1 || in.choices[0].InboundEndpointID == "" || in.choices[0].OutboundEndpointID != b.ID {
				t.Fatal("deterministic endpoint resolution changed", in.choices, err)
			}
			if test.source != "" && in.sources["rule"].ID != test.source {
				t.Fatal("explicit host selection was replaced by a node default")
			}
		})
	}
}

func TestRoutingProofDoesNotInventAnEdgeBetweenDifferentInbounds(t *testing.T) {
	a := Endpoint{NodeID: "a", InboundTag: "primary"}
	b := Endpoint{NodeID: "b", InboundTag: "primary"}
	b2 := Endpoint{NodeID: "b", InboundTag: "alternate"}
	c := Endpoint{NodeID: "c", InboundTag: "primary"}
	graph, err := routingmatch.Build([]routingmatch.Rule{{ID: "ab", From: routingVertex(a), To: routingVertex(b)}, {ID: "bc", From: routingVertex(b2), To: routingVertex(c)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := graph.Paths()
	if err != nil {
		t.Fatal(err)
	}
	in := routingInput{vertices: map[string]string{routingVertex(a): "a", routingVertex(b): "b", routingVertex(b2): "b", routingVertex(c): "c"}}
	for _, path := range in.publicPaths(paths) {
		if slices.Equal(path.Nodes, []string{"a", "b", "c"}) {
			t.Fatal("different listeners were treated as a connected path")
		}
	}
}
