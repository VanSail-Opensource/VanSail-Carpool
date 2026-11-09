package connections

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type legacyExpiryRepository struct {
	expiryRepo
	aliases []secret.ConnectionIPOwnerAlias
}

func (r *legacyExpiryRepository) ConnectionIPOwnerAliases(context.Context, string) ([]secret.ConnectionIPOwnerAlias, error) {
	return r.aliases, nil
}

func TestLegacyExpiryPreservesEvidenceUntilScopedNodeAcceptance(t *testing.T) {
	ctx := context.Background()
	at := time.Now().UTC()
	vault, err := secret.NewVault([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := vault.Encrypt("connection-ip-block:legacy:node:digest", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	repo := &legacyExpiryRepository{expiryRepo: expiryRepo{block: IPBlockRecord{ID: "block", UserID: "current", NodeUUID: "node", IPDigest: "digest", SealedIP: sealed, ExpiresAt: at.Add(-time.Hour)}}, aliases: []secret.ConnectionIPOwnerAlias{{OldOwnerID: "legacy", OwnerID: "current"}}}
	remote := &legacyScopedRemote{refuse: true}
	worker := NewExpiryWorker(repo, remote, emby.NewSecretBox(vault))
	job := model.OutboxJob{Kind: BlockExpiryOutboxKind, Payload: `{"blockId":"block"}`}
	if err = worker.HandleOutbox(ctx, job); err == nil || repo.deleted {
		t.Fatal("native refusal discarded retained target", err)
	}
	remote.refuse = false
	if err = worker.HandleOutbox(ctx, job); err != nil || !repo.deleted || remote.scoped != 1 || remote.owner != "current" {
		t.Fatal("recovered target could not finish scoped expiry", err)
	}
	if len(remote.events) != 0 || repo.block.SealedIP != sealed {
		t.Fatal("expiry changed ciphertext or invoked global unblock")
	}
}
