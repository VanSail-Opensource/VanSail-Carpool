package xnode

import (
	"errors"
	"slices"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHostOriginsSkipUnusedSettingsAndClearRemovedHosts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint *Endpoint
	}{
		{name: "no endpoint"},
		{name: "IPv4", endpoint: &Endpoint{Enabled: true, Address: "8.8.8.8"}},
		{name: "IPv4 trailing dot", endpoint: &Endpoint{Enabled: true, Address: "8.8.8.8."}},
		{name: "IPv6", endpoint: &Endpoint{Enabled: true, Address: "2606:4700:4700::1111"}},
		{name: "mapped IPv4", endpoint: &Endpoint{Enabled: true, Address: "::ffff:8.8.8.8"}},
		{name: "disabled hostname", endpoint: &Endpoint{Address: "HOST.EXAMPLE."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			master, repo := hostOriginReadFixture(t)
			repo.nodes = []Node{{ID: "unused"}}
			repo.nodeErrors["unused"] = errors.New("unused settings unavailable")
			if tc.endpoint != nil {
				endpoint := *tc.endpoint
				endpoint.NodeID = "unused"
				repo.endpoints = []Endpoint{endpoint}
			}
			master.managedNodeIPs = map[string][]string{"unused": {"8.8.8.8"}}
			master.ddnsNodeIPs = map[string][]string{"unused": {"9.9.9.9"}}
			master.hostOrigins = map[string]hostOrigin{"removed.example": retainedHostOrigin(master, "8.8.8.8")}
			master.managedHostIPs = []string{"8.8.8.8"}
			if err := master.refreshHostOrigins(t.Context()); err != nil {
				t.Fatal("irrelevant node settings blocked origin maintenance", err)
			}
			if repo.endpointReads != 1 || repo.nodesReads != 1 || len(repo.nodeReads) != 0 || len(repo.bindingReads) != 0 {
				t.Fatal("inventory was skipped or unused settings were loaded", repo)
			}
			if len(master.hostOrigins) != 0 || len(master.managedHostIPs) != 0 {
				t.Fatal("zero-host maintenance retained obsolete DNS protection")
			}
			if !slices.Equal(master.managedNodeIPs["unused"], []string{"8.8.8.8"}) || !slices.Equal(master.ddnsNodeIPs["unused"], []string{"9.9.9.9"}) {
				t.Fatal("DNS cleanup mutated authenticated origin evidence")
			}
		})
	}
}

func TestHostOriginsRetainUsedSettingsFailures(t *testing.T) {
	storageFailure := errors.New("hostname settings unavailable")
	for _, source := range []string{"node", "settings", "bindings"} {
		t.Run(source, func(t *testing.T) {
			master, repo := hostOriginReadFixture(t)
			repo.nodes = []Node{{ID: "used"}}
			repo.endpoints = []Endpoint{{NodeID: "used", Enabled: true, Address: "HOST.EXAMPLE."}}
			saveHostOriginReadSettings(t, master, repo, "used", NodeSettings{DDNS: []wire.DDNSJob{{ID: "wan", IPv4: "8.8.8.8"}}})
			want := storageFailure
			switch source {
			case "node":
				repo.nodeErrors["used"] = want
			case "settings":
				cipher, err := master.service.vault.Encrypt("xnode.node.used", `[]`)
				if err != nil {
					t.Fatal(err)
				}
				repo.stored["used"] = Node{ID: "used", SettingsCipher: cipher}
				want = errNodeSettingsNotObject
			case "bindings":
				repo.bindingErrors["used"] = want
			}
			master.hostOrigins = map[string]hostOrigin{"host.example": retainedHostOrigin(master, "8.8.8.8")}
			master.managedHostIPs = []string{"8.8.8.8"}
			if err := master.refreshHostOrigins(t.Context()); !errors.Is(err, want) {
				t.Fatal("used hostname settings failure was hidden", err)
			}
			if repo.nodeReads["used"] != 1 || len(master.hostOrigins) != 1 || !slices.Equal(master.managedHostIPs, []string{"8.8.8.8"}) {
				t.Fatal("failed settings read changed the previously published protection")
			}
		})
	}
}

func TestHostOriginsRetainInventoryFailuresWithoutHostnames(t *testing.T) {
	failure := errors.New("inventory unavailable")
	for _, source := range []string{"endpoints", "nodes"} {
		t.Run(source, func(t *testing.T) {
			master, repo := hostOriginReadFixture(t)
			if source == "endpoints" {
				repo.endpointErr = failure
			} else {
				repo.nodesErr = failure
			}
			if err := master.refreshHostOrigins(t.Context()); !errors.Is(err, failure) {
				t.Fatal("empty hostname inventory suppressed its read failure", err)
			}
			if repo.endpointReads != 1 || source == "nodes" && repo.nodesReads != 1 || len(repo.nodeReads) != 0 {
				t.Fatal("inventory failure changed the read sequence", repo)
			}
		})
	}
}
