package remnawave

import (
	"encoding/json"
	"testing"
)

func TestHostAccessAcrossLegacyAndCurrentSchemas(t *testing.T) {
	for _, scenario := range []struct {
		raw     string
		allowed bool
	}{
		{`{"excludedInternalSquads":["other"]}`, true},
		{`{"excludedInternalSquads":["paid"]}`, false},
		{`{"internalSquads":{"mode":"ALLOW_ONLY","squads":["paid"]}}`, true},
		{`{"internalSquads":{"mode":"ALLOW_ONLY","squads":["other"]}}`, false},
		{`{"internalSquads":{"mode":"EXCLUDE","squads":["paid"]}}`, false},
		{`{"internalSquads":{"mode":"UNKNOWN","squads":[]}}`, false},
	} {
		var host Host
		if err := json.Unmarshal([]byte(scenario.raw), &host); err != nil {
			t.Fatal(err)
		}
		if got := host.AllowsSquad("paid"); got != scenario.allowed {
			t.Fatalf("%s: allowed=%v", scenario.raw, got)
		}
	}
}

func TestHostWithoutExplicitNodesResolvesOnlyItsActiveInbound(t *testing.T) {
	id := "inbound"
	host := Host{}
	host.Inbound.ConfigProfileInboundUUID = &id
	nodes := []Node{{UUID: "matched"}, {UUID: "other"}}
	nodes[0].ConfigProfile.ActiveInbounds = []NodeInbound{{UUID: id}}
	nodes[1].ConfigProfile.ActiveInbounds = []NodeInbound{{UUID: "different"}}
	got := host.HostNodeIDs(nodes)
	if len(got) != 1 || got[0] != "matched" {
		t.Fatalf("resolved nodes=%v", got)
	}
	host.Nodes = []string{"explicit"}
	if got = host.HostNodeIDs(nodes); len(got) != 1 || got[0] != "explicit" {
		t.Fatalf("explicit relation changed=%v", got)
	}
}
