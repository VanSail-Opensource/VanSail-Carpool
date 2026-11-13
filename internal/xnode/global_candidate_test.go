package xnode

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type obsoleteGlobalSettings struct{ cacheTestSettings }

func (obsoleteGlobalSettings) Optional(_ context.Context, key string) (string, error) {
	if key == GlobalSettingKey {
		return "invalid retained global document", nil
	}
	return "", nil
}

func TestFleetCompileUsesPrivateCandidateWithoutReadingObsoleteSettings(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &cacheTestRepository{revision: 1, node: Node{ID: "node"}}
	service := NewService(repo, obsoleteGlobalSettings{}, vault, nil, nil)
	repo.config.DocumentCipher, err = vault.Encrypt("xnode.config.", `{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	repo.node.SettingsCipher, err = service.seal("xnode.node.node", NodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	candidate := DefaultGlobal()
	candidate.Telemetry.ReportSeconds = 10
	candidate.BlockedDomains = []string{"domain:example.org"}
	config, err := service.compile(context.Background(), "node", nil, nil, &candidate, nil)
	if err != nil || config.Telemetry.ReportSeconds != 10 || len(config.BlockedDomains) != 1 || config.BlockedDomains[0] != candidate.BlockedDomains[0] {
		t.Fatal("candidate compilation consulted obsolete global settings", config.Telemetry, err)
	}
	config.BlockedDomains[0] = "full:changed.invalid"
	if candidate.BlockedDomains[0] != "domain:example.org" {
		t.Fatal("compiled document aliased the candidate")
	}
	if _, err := service.Global(context.Background()); err == nil {
		t.Fatal("validation published its candidate to the serving service")
	}
}
