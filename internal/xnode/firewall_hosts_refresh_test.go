package xnode

import (
	"errors"
	"slices"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHostOriginsKeepFreshConfiguredTrustWithoutTelemetry(t *testing.T) {
	master, repo := hostOriginReadFixture(t)
	repo.nodes = []Node{{ID: "used"}, {ID: "unused"}}
	repo.nodeErrors["unused"] = errors.New("literal-only settings unavailable")
	repo.endpoints = []Endpoint{
		{NodeID: "used", Enabled: true, Hidden: true, Address: "HOST.EXAMPLE."},
		{NodeID: "used", Enabled: true, Address: "host.example"},
		{NodeID: "unused", Enabled: true, Address: "1.1.1.1"},
	}
	settings := NodeSettings{DDNS: []wire.DDNSJob{{ID: "wan", IPv4: "8.8.8.8"}}}
	saveHostOriginReadSettings(t, master, repo, "used", settings)
	previous := retainedHostOrigin(master, "8.8.8.8", "9.9.9.9", "1.1.1.1")
	master.hostOrigins = map[string]hostOrigin{"host.example": previous}
	for turn, want := range []string{"8.8.8.8", "9.9.9.9"} {
		settings.DDNS[0].IPv4 = want
		saveHostOriginReadSettings(t, master, repo, "used", settings)
		if err := master.refreshHostOrigins(t.Context()); err != nil {
			t.Fatal("configured DDNS trust required telemetry or an enabled node", err)
		}
		if !slices.Equal(master.managedHostIPs, []string{want}) || repo.nodeReads["used"] != turn+1 || repo.bindingReads["used"] != turn+1 || repo.nodeReads["unused"] != 0 {
			t.Fatal("origin evidence was stale, broadened or redundantly loaded", master.managedHostIPs, repo.nodeReads)
		}
		current := master.hostOrigins["host.example"]
		if !current.refreshAt.Equal(previous.refreshAt) || !current.validUntil.Equal(previous.validUntil) || !slices.Equal(current.addresses, previous.addresses) {
			t.Fatal("fresh settings changed the existing DNS cache schedule")
		}
	}
}

func TestHostOriginsKeepNormalizedHostnameTrustUnionAndScope(t *testing.T) {
	master, repo := hostOriginReadFixture(t)
	repo.nodes = []Node{{ID: "first"}, {ID: "second"}, {ID: "other"}}
	repo.endpoints = []Endpoint{
		{NodeID: "first", Enabled: true, Address: "SHARED.EXAMPLE."},
		{NodeID: "second", Enabled: true, Address: "shared.example"},
		{NodeID: "other", Address: "shared.example"},
	}
	for _, node := range []string{"first", "second"} {
		saveHostOriginReadSettings(t, master, repo, node, NodeSettings{})
	}
	repo.nodeErrors["other"] = errors.New("disabled endpoint settings unavailable")
	master.managedNodeIPs = map[string][]string{"first": {"8.8.8.8"}, "other": {"1.1.1.1"}}
	master.ddnsNodeIPs = map[string][]string{"second": {"9.9.9.9"}}
	master.hostOrigins = map[string]hostOrigin{"shared.example": retainedHostOrigin(master, "8.8.8.8", "9.9.9.9", "1.1.1.1")}
	if err := master.refreshHostOrigins(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(master.managedHostIPs, []string{"8.8.8.8", "9.9.9.9"}) || repo.nodeReads["first"] != 1 || repo.nodeReads["second"] != 1 || repo.nodeReads["other"] != 0 {
		t.Fatal("shared hostname lost its exact enabled-node trust scope", master.managedHostIPs, repo.nodeReads)
	}
	if !slices.Equal(master.managedNodeIPs["first"], []string{"8.8.8.8"}) || !slices.Equal(master.ddnsNodeIPs["second"], []string{"9.9.9.9"}) {
		t.Fatal("hostname aggregation mutated authenticated evidence")
	}
}
