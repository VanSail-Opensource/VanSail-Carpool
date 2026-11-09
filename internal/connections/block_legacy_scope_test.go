package connections

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type legacyBlockRepository struct {
	*blockWorkerRepo
	aliases []secret.ConnectionIPOwnerAlias
}

func (r *legacyBlockRepository) ConnectionIPOwnerAliases(context.Context, string) ([]secret.ConnectionIPOwnerAlias, error) {
	return r.aliases, nil
}

type legacyScopedRemote struct {
	blockWorkerRemote
	owner, ip, node string
	scoped          int
	refuse          bool
}

func (r *legacyScopedRemote) ControlMemberIP(_ context.Context, owner, ip, node string, drop bool, seconds int, unblock bool) error {
	if r.refuse || drop || seconds != 0 || !unblock {
		return errors.New("native scope refused")
	}
	r.owner, r.ip, r.node = owner, ip, node
	r.scoped++
	return nil
}

func TestLegacyIPTargetRetainsCurrentOwnerScopedNativeRelease(t *testing.T) {
	for _, test := range []struct {
		name            string
		foreign, refuse bool
	}{
		{"matching_alias", false, false}, {"foreign_alias", true, false}, {"native_scope_fence", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			at := time.Now().UTC()
			base, _, _ := newBlockWorkerFixture(at)
			vault, err := secret.NewVault([]byte(strings.Repeat("k", 32)))
			if err != nil {
				t.Fatal(err)
			}
			base.operation.Receipt.Kind = UnblockOperationKind
			base.block.SealedIP, err = vault.Encrypt("connection-ip-block:old-owner:"+base.block.NodeUUID+":"+base.block.IPDigest, "203.0.113.8")
			if err != nil {
				t.Fatal(err)
			}
			sealed := base.block.SealedIP
			alias := secret.ConnectionIPOwnerAlias{OldOwnerID: "old-owner", OwnerID: "owner"}
			if test.foreign {
				alias.OwnerID = "another"
			}
			repo := &legacyBlockRepository{blockWorkerRepo: base, aliases: []secret.ConnectionIPOwnerAlias{alias}}
			remote := &legacyScopedRemote{refuse: test.refuse}
			worker := NewUnblockWorker(repo, remote, emby.NewSecretBox(vault))
			err = worker.HandleProviderOperation(context.Background(), repo.operation, model.OutboxJob{})
			if test.foreign {
				if err == nil || repo.deleted || remote.scoped != 0 {
					t.Fatal("foreign alias released target", err)
				}
			} else if test.refuse {
				if err != nil || repo.deleted || repo.operation.Receipt.Status != "pending_review" {
					t.Fatal("native refusal was bypassed", err)
				}
			} else if err != nil || !repo.deleted || remote.scoped != 1 || remote.owner != "owner" || remote.node != base.block.NodeUUID || remote.ip != "203.0.113.8" {
				t.Fatal("historical scope did not preserve current native subject", err)
			}
			if len(remote.events) != 0 || base.block.SealedIP != sealed {
				t.Fatal("legacy resolver changed ciphertext or called global unblock")
			}
		})
	}
}
