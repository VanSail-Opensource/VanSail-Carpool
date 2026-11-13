package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type boundaryCompileRepository struct {
	cacheTestRepository
	boundary time.Time
	lookup   func()
}

func (r *boundaryCompileRepository) XNodeCompileVersion(_ context.Context, at time.Time) (int64, time.Time, error) {
	if r.lookup != nil {
		r.lookup()
	}
	if at.Before(r.boundary) {
		return r.revision, r.boundary, nil
	}
	return r.revision, time.Time{}, nil
}

func TestCompileCannotPublishAcrossPaidAccessBoundary(t *testing.T) {
	for _, stage := range []string{"cached lookup", "compilation", "final lookup"} {
		t.Run(stage, func(t *testing.T) {
			vault, err := secret.NewVault(make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			repo := &boundaryCompileRepository{cacheTestRepository: cacheTestRepository{revision: 1, node: Node{ID: "node"}}, boundary: now.Add(time.Second)}
			service := NewService(repo, cacheTestSettings{}, vault, nil, nil)
			service.now = func() time.Time { return now }
			repo.config.DocumentCipher, err = vault.Encrypt("xnode.config.", `{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`)
			if err != nil {
				t.Fatal(err)
			}
			repo.node.SettingsCipher, err = service.seal("xnode.node.node", NodeSettings{})
			if err != nil {
				t.Fatal(err)
			}
			master := NewMaster(service, nil)
			previous := &compiledState{version: 0, until: repo.boundary}
			peer := &agentPeer{nodeID: "node", compiled: previous}
			switch stage {
			case "cached lookup":
				previous.version = repo.revision
				repo.lookup = func() { now = repo.boundary }
			case "compilation":
				repo.beforeRead = func() { now = repo.boundary }
			case "final lookup":
				calls := 0
				repo.lookup = func() {
					calls++
					if calls == 2 {
						now = repo.boundary
					}
				}
			}
			if _, err = master.cachedConfiguration(context.Background(), peer, false); !errors.Is(err, ErrConflict) {
				t.Fatal("elapsed access window was returned as current", err)
			}
			if peer.compiled != previous || peer.compileFailure != nil {
				t.Fatal("expired candidate replaced the cache or delayed the next current compile")
			}
			repo.lookup, repo.beforeRead = nil, nil
			if _, err = master.cachedConfiguration(context.Background(), peer, false); err != nil {
				t.Fatal("new access window could not compile immediately", err)
			}
		})
	}
}
