package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"io"
	"log/slog"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestNativeValidationCachePreservesSectionsAndAgentDependencies(t *testing.T) {
	service := &Service{compilation: &validationCache{}}
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[{"tag":"paid","listen":"127.0.0.1","port":12345,"protocol":"vless","settings":{"decryption":"none"}}],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`), Users: []wire.User{{ID: 1, UUID: "550e8400-e29b-41d4-a716-446655440000", InboundTags: []string{"paid"}}}}
	if result := service.validateCompiledCached("node-a", config); !result.Valid {
		t.Fatal(result)
	}
	entry := service.compilation.entry("node:node-a")
	before := entry.validator.Counts()
	for i := 0; i < 20; i++ {
		copy := wire.Clone(config)
		copy.Users[0].BlockNew = i%2 == 0
		copy.Users[0].SpeedLimitMbps = i
		if result := service.validateCompiledCached("node-a", copy); !result.Valid {
			t.Fatal(result)
		}
	}
	if after := entry.validator.Counts(); after != before {
		t.Fatalf("registry edits recompiled native sections: before=%+v after=%+v", before, after)
	}
	config.Users[0].UUID = "550e8400-e29b-41d4-a716-446655440001"
	if result := service.validateCompiledCached("node-a", config); !result.Valid {
		t.Fatal(result)
	}
	after := entry.validator.Counts()
	if after.Inbounds-before.Inbounds != 1 || after.Native != before.Native || after.Outbounds != before.Outbounds {
		t.Fatalf("credential change rebuilt unrelated sections %+v", after)
	}
	var root map[string]any
	_ = json.Unmarshal(config.Xray, &root)
	root["geodata"] = map[string]any{"assets": []any{map[string]any{"file": "geosite.dat", "url": "https://example.com/geosite.dat"}}}
	config.Xray, _ = json.Marshal(root)
	if result := service.validateCompiledCached("node-a", config); !result.Valid || !result.NeedsAgentValidation {
		t.Fatal("cached proof hid node-local dependency", result)
	}
	if result := service.validateCompiledCached("node-b", config); !result.Valid {
		t.Fatal(result)
	}
	if service.compilation.entry("node:node-b") == entry {
		t.Fatal("node validation contexts shared")
	}
}

func TestValidationCacheBoundedAndInvalidCandidatesDoNotPoisonProof(t *testing.T) {
	service := &Service{compilation: &validationCache{}}
	valid := wire.Config{Xray: json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`), Users: []wire.User{}}
	if !service.validateCompiledCached("node", valid).Valid {
		t.Fatal("valid document rejected")
	}
	invalid := wire.Clone(valid)
	invalid.Xray = json.RawMessage(`{"inbounds":false}`)
	if service.validateCompiledCached("node", invalid).Valid {
		t.Fatal("invalid candidate accepted")
	}
	if !service.validateCompiledCached("node", valid).Valid {
		t.Fatal("failed candidate poisoned last proof")
	}
	for i := 0; i < 200; i++ {
		service.compilation.entry(fmt.Sprint(i))
	}
	if len(service.compilation.entries) > 128 {
		t.Fatal("validation cache grew without bound")
	}
}

func TestCoarseEpochKeepsPreparedWireForUnchangedNode(t *testing.T) {
	service, repo, peer, master := compileCacheFixture(t)
	_ = service
	if _, err := master.cachedConfiguration(context.Background(), peer, false); err != nil {
		t.Fatal(err)
	}
	saved := peer.compiled
	prepared, err := preparePeerWire(peer, saved.config, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo.revision++
	if _, err = master.cachedConfiguration(context.Background(), peer, false); err != nil {
		t.Fatal(err)
	}
	if peer.compiled != saved {
		t.Fatal("unrelated epoch discarded effective node snapshot")
	}
	next, err := preparePeerWire(peer, peer.compiled.config, nil)
	if err != nil || next != prepared {
		t.Fatal("unchanged wire document reprocessed", err)
	}
}

func compileCacheFixture(t *testing.T) (*Service, *cacheTestRepository, *agentPeer, *Master) {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &cacheTestRepository{revision: 1, node: Node{ID: "node"}}
	service := NewService(repo, cacheTestSettings{}, vault, nil, nil)
	repo.config.DocumentCipher, err = vault.Encrypt("xnode.config.", `{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	repo.node.SettingsCipher, err = service.seal("xnode.node.node", NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	master := NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return service, repo, &agentPeer{nodeID: "node"}, master
}
