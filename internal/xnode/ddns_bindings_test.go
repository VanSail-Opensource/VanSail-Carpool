package xnode

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type ddnsBindingTestRepository struct {
	Repository
	nodes    map[string]Node
	bindings map[string]DDNSHostBinding
	role     string
}

func (r *ddnsBindingTestRepository) UserByID(context.Context, string) (model.User, error) {
	return model.User{ID: "admin", Role: r.role}, nil
}
func (r *ddnsBindingTestRepository) XNodeConfiguration(context.Context, string) (Configuration, error) {
	return Configuration{ID: "config"}, nil
}
func (r *ddnsBindingTestRepository) XNodeNode(_ context.Context, id string) (Node, error) {
	if node, ok := r.nodes[id]; ok {
		return node, nil
	}
	return Node{}, ErrNotFound
}
func (r *ddnsBindingTestRepository) XNodeEndpoints(context.Context) ([]Endpoint, error) {
	return []Endpoint{{ID: "host-v4", NodeID: "node"}, {ID: "host-v6", NodeID: "node"}, {ID: "foreign-host", NodeID: "other"}}, nil
}
func (r *ddnsBindingTestRepository) XNodeDDNSBindings(context.Context, string) (map[string]DDNSHostBinding, error) {
	bindings := make(map[string]DDNSHostBinding, len(r.bindings))
	for id, binding := range r.bindings {
		bindings[id] = binding
	}
	return bindings, nil
}
func (r *ddnsBindingTestRepository) SaveXNodeNodeDDNS(_ context.Context, node Node, bindings map[string]DDNSHostBinding) error {
	r.nodes[node.ID] = node
	r.bindings = bindings
	return nil
}

func TestDDNSBindingsKeepDiscoveryAndHostOwnershipSeparate(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &ddnsBindingTestRepository{nodes: map[string]Node{}, bindings: map[string]DDNSHostBinding{}, role: "admin"}
	s := NewService(repo, cacheTestSettings{}, vault, nil, nil)
	ctx := context.Background()
	node := Node{ID: "node", Name: "Node", ConfigID: "config", Enabled: true, Multiplier: 1}
	settings := NodeSettings{DDNS: []wire.DDNSJob{{ID: "wan", Provider: wire.BuiltinDDNSProvider, Generation: "caller-value", IPv4: "auto", IPv6: "auto", Interface: "eth0"}}, DDNSBindings: map[string]DDNSHostBinding{"wan": {IPv4HostID: "host-v4", IPv6HostID: "host-v6", ConfiguredBy: "forged"}}}
	saved, err := s.SaveNodeAs(ctx, node, settings, "admin")
	if err != nil {
		t.Fatal(err)
	}
	var stored NodeSettings
	if err = s.open("xnode.node.node", saved.SettingsCipher, &stored); err != nil || stored.DDNSBindings != nil || stored.DDNS[0].Generation != "" {
		t.Fatal("duplicate binding/generation persistence", stored, err)
	}
	_, restored, err := s.Node(ctx, node.ID)
	if err != nil || restored.DDNSBindings["wan"].ConfiguredBy != "admin" {
		t.Fatal("caller forged audit ownership", restored, err)
	}
	jobs := ddnsJobsForNode(node.ID, restored, nil)
	raw, _ := json.Marshal(jobs)
	if len(jobs[0].Generation) != 64 || strings.Contains(string(raw), "host-v4") || strings.Contains(string(raw), "admin") {
		t.Fatal("panel references entered discovery config", string(raw))
	}
	oldGeneration := jobs[0].Generation
	binding := restored.DDNSBindings["wan"]
	binding.IPv6HostID = "host-v4"
	restored.DDNSBindings["wan"] = binding
	if _, err = s.SaveNodeAs(ctx, node, restored, "admin"); err == nil {
		t.Fatal("same host used for both families")
	}
	binding.IPv6HostID = "foreign-host"
	restored.DDNSBindings["wan"] = binding
	if _, err = s.SaveNodeAs(ctx, node, restored, "admin"); err == nil {
		t.Fatal("foreign node host accepted")
	}
	_, restored, _ = s.Node(ctx, node.ID)
	restored.DDNSBindings = nil
	if _, err = s.SaveNode(ctx, node, restored); err != nil || repo.bindings["wan"].IPv6HostID != "host-v6" {
		t.Fatal("old client omission erased bindings", err)
	}
	_, restored, _ = s.Node(ctx, node.ID)
	restored.DDNS[0].Interface = "ens3"
	if ddnsJobsForNode(node.ID, restored, nil)[0].Generation == oldGeneration {
		t.Fatal("changed discovery kept an old generation")
	}
	repo.role = "user"
	if _, err = s.SaveNodeAs(ctx, node, restored, "admin"); err == nil {
		t.Fatal("nonadministrator configured node automation")
	}
}

func TestDDNSCapabilitiesPreserveLegacyAuthorizationAndCredentials(t *testing.T) {
	config := wire.Config{Users: []wire.User{{ID: 1, UUID: "credential"}}, DDNS: []wire.DDNSJob{{ID: "internal", Provider: wire.BuiltinDDNSProvider, Generation: "generation"}, {ID: "external", Provider: "cloudflare", Generation: "generation"}}}
	legacy := ddnsForCapabilities(config, nil, false)
	if len(legacy.DDNS) != 1 || legacy.DDNS[0].ID != "external" || legacy.DDNS[0].Generation != "" || legacy.Users[0].UUID != "credential" {
		t.Fatal("new feature interrupted legacy traffic", legacy)
	}
	capabilities := []string{wire.BuiltinDDNSCapability, wire.DDNSChangesCapability}
	if len(ddnsForCapabilities(config, capabilities, false).DDNS) != 1 {
		t.Fatal("hello alone enabled an unnegotiated extension")
	}
	modern := ddnsForCapabilities(config, capabilities, true)
	if len(modern.DDNS) != 2 || modern.DDNS[0].Generation == "" || config.DDNS[0].Generation != "generation" {
		t.Fatal("modern config lost generation or mutated cache", modern)
	}
}
