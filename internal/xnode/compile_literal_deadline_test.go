package xnode

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func TestRoutingLiteralDoesNotExpireUnchangedCompiledInventory(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &cacheTestRepository{revision: 1, node: Node{ID: "node"}}
	service := NewService(repo, cacheTestSettings{}, vault, nil, nil)
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	repo.config.DocumentCipher, err = vault.Encrypt("xnode.config.", `{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	repo.node.SettingsCipher, err = service.seal("xnode.node.node", NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	// A routing destination elsewhere in the fleet shares this runtime cache.
	// It must not impose an artificial DNS deadline on any compiled node.
	if _, err = service.resolveRoutingPeer(t.Context(), "8.8.8.8", "destination"); err != nil {
		t.Fatal(err)
	}
	master := NewMaster(service, nil)
	peer := &agentPeer{nodeID: "node"}
	if _, err = master.cachedConfiguration(t.Context(), peer, false); err != nil {
		t.Fatal(err)
	}
	previous := peer.compiled
	if !previous.until.IsZero() {
		t.Fatal("literal routing destination introduced a compiled-state expiry", previous.until)
	}
	now = now.Add(6 * time.Minute)
	if _, err = master.cachedConfiguration(t.Context(), peer, false); err != nil || repo.reads != 1 || peer.compiled != previous {
		t.Fatal("literal routing destination forced an unchanged inventory recompile", err, repo.reads)
	}
	repo.revision++
	if _, err = master.cachedConfiguration(t.Context(), peer, false); err != nil || repo.reads != 2 {
		t.Fatal("authoritative compile revision stopped invalidating the inventory", err, repo.reads)
	}
}
