package xnode

import "testing"

func TestRoutingAccessPrunesLostNodeAndEndpointReferences(t *testing.T) {
	a := Endpoint{ID: "a", NodeID: "a"}
	b := Endpoint{ID: "b", NodeID: "b"}
	c := Endpoint{ID: "c", NodeID: "c"}
	in := routingInput{endpoints: map[string]Endpoint{"a": a, "b": b}, byEndpoint: map[string]Endpoint{"a": a, "b": b}}
	choices := []RoutingChoice{
		{ID: "keep", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "a", OutboundEndpointID: "b"},
		{ID: "lost-node", InboundNodeID: "b", OutboundNodeID: "c", InboundEndpointID: "b", OutboundEndpointID: "c"},
		{ID: "lost-host", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "retired-a", OutboundEndpointID: "b"},
		{ID: "legacy-default", InboundNodeID: "a", OutboundNodeID: "b"},
	}
	kept := accessibleRoutingChoices(choices, in)
	if len(kept) != 2 || kept[0].ID != "keep" || kept[1].ID != "legacy-default" {
		t.Fatal(kept)
	}
	in.endpoints["c"], in.byEndpoint["c"] = c, c
	if len(accessibleRoutingChoices(choices, in)) != 3 {
		t.Fatal("authorized node removed")
	}
}
