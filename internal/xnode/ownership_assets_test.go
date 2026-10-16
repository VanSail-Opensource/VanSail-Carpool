package xnode_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestLegacyOwnershipStagesDataWithoutAuthorizingListeners(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	if err := f.store.SaveXNodeMigration(ctx, xnode.MigrationState{Phase: "preview"}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SaveXNodeMigrationNode(ctx, xnode.MigrationNode{NodeID: f.node.ID, LegacyNodeID: f.node.ID, Phase: "legacy"}); err != nil {
		t.Fatal(err)
	}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	settings := &fixtureSettings{raw: `{"geoSiteUrl":"https://assets.example/geosite.dat","geoSiteHash":"` + hash + `","blockedPorts":"22","updates":{"enabled":true},"torrent":{"enabled":true}}`}
	service := xnode.NewService(f.store, settings, vault, nil, nil)
	config, err := service.Compile(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Inbounds []any `json:"inbounds"`
		Geodata  struct {
			Assets []struct {
				File string `json:"file"`
				URL  string `json:"url"`
			} `json:"assets"`
		} `json:"geodata"`
	}
	if err = json.Unmarshal(config.Xray, &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Inbounds) != 0 || len(config.Users) != 0 || config.Firewall.Enabled || config.Updates.Enabled || config.Torrent.Enabled || len(config.BlockedPorts) != 0 {
		t.Fatal("legacy asset staging granted native access or applied host policy")
	}
	if len(root.Geodata.Assets) != 1 || root.Geodata.Assets[0].File != "geosite.dat" || root.Geodata.Assets[0].URL != "https://assets.example/geosite.dat" || config.AssetHashes["geosite.dat"] != hash {
		t.Fatal("legacy candidate cannot obtain its verified data before handover")
	}
}
