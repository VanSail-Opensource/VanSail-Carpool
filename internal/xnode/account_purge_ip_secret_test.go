package xnode

import (
	"context"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type purgeLegacyIPRepository struct {
	Repository
	aliases []secret.ConnectionIPOwnerAlias
	lookups int
}

func (r *purgeLegacyIPRepository) ConnectionIPOwnerAliases(context.Context, string) ([]secret.ConnectionIPOwnerAlias, error) {
	r.lookups++
	return r.aliases, nil
}

func TestPurgeLegacyIPReaderNeverFollowsForeignOwnerOrChangedTarget(t *testing.T) {
	vault, err := secret.NewVault([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := vault.Encrypt("connection-ip-block:legacy:node:digest", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	repo := &purgeLegacyIPRepository{aliases: []secret.ConnectionIPOwnerAlias{{OldOwnerID: "legacy", OwnerID: "current"}}}
	s := NewService(repo, nil, vault, nil, nil)
	block := connections.IPBlockRecord{UserID: "current", NodeUUID: "node", IPDigest: "digest", SealedIP: sealed}
	plain, err := s.openAccountPurgeIPTarget(context.Background(), "current", block)
	clear(plain)
	if err != nil || repo.lookups != 1 {
		t.Fatal("retained scope unavailable", err)
	}
	repo.lookups = 0
	if _, err = s.openAccountPurgeIPTarget(context.Background(), "foreign", block); err == nil || repo.lookups != 0 {
		t.Fatal("purge owner fence was bypassed", err)
	}
	for _, mutate := range []func(*connections.IPBlockRecord){func(p *connections.IPBlockRecord) { p.NodeUUID = "another" }, func(p *connections.IPBlockRecord) { p.IPDigest = "another" }} {
		changed := block
		mutate(&changed)
		if _, err = s.openAccountPurgeIPTarget(context.Background(), "current", changed); err == nil {
			t.Fatal("changed block target authenticated")
		}
	}
	if block.SealedIP != sealed {
		t.Fatal("read compatibility rewrote stored evidence")
	}
}
