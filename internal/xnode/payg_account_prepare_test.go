package xnode

import (
	"context"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type atomicPAYGRepository struct {
	*paygLeaseRepository
	prepared PAYGPreparation
	calls    int
}

func (r *atomicPAYGRepository) XNodeIdentities(context.Context) ([]Identity, error) {
	// A compile can capture its inventory before atomic preparation creates the
	// principal. The real repository's direct fallback loads the new identity.
	return []Identity{}, nil
}

func (r *atomicPAYGRepository) UserByID(context.Context, string) (model.User, error) {
	return model.User{ID: "owner", AccountStatus: "active", AccountGeneration: 7}, nil
}
func (r *atomicPAYGRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	return Identity{CredentialsCipher: "already-persisted"}, nil
}
func (r *atomicPAYGRepository) PrepareXNodePAYGAccount(_ context.Context, input PAYGPreparation) (PAYGAccount, error) {
	r.prepared = input
	r.calls++
	return PAYGAccount{ProxyUserID: 4503599627370496, UserID: input.OwnerID, GroupID: input.GroupID, NodeID: input.NodeID, PeriodID: input.PeriodID}, nil
}

func TestCompileScopeUsesAtomicPAYGPreparationAndKeepsExistingFastPath(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &atomicPAYGRepository{paygLeaseRepository: &paygLeaseRepository{}}
	service := NewService(repo, nil, vault, nil, nil).withCompileReads("node")
	if err = service.ensurePAYGAccount(context.Background(), "owner", "group", "node", 100); err != nil {
		t.Fatal(err)
	}
	input := repo.prepared
	if repo.calls != 1 || input.OwnerID != "owner" || input.AccountGeneration != 7 || input.ServiceName != PAYGAccountServiceName("owner", "group", "node") || !strings.HasPrefix(input.PeriodID, "payg-") {
		t.Fatal("compile wrapper bypassed atomic repository", repo.calls, input.AccountGeneration)
	}
	if _, err = vault.Decrypt("xnode.identity.service."+input.ServiceName, input.CredentialsCipher); err != nil {
		t.Fatal("preparation cipher lost original service scope", err)
	}
	repo.accounts = []PAYGAccount{{ProxyUserID: 4503599627370496, UserID: "owner", GroupID: "group", NodeID: "node", PeriodID: input.PeriodID}}
	service.vault = nil // Existing accounts must not mint another credential/period.
	if err = service.ensurePAYGAccount(context.Background(), "owner", "group", "node", 100); err != nil || repo.calls != 1 {
		t.Fatal("existing account performed new preparation", repo.calls, err)
	}
}
