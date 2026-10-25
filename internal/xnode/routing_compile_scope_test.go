package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type routingScopeRepository struct {
	Repository
	routingRepository
	plans         []RoutingPlan
	identityReads int
	identityErr   error
}

func (r *routingScopeRepository) XNodeRoutingPlans(context.Context) ([]RoutingPlan, error) {
	return r.plans, nil
}

func (r *routingScopeRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	r.identityReads++
	return Identity{}, r.identityErr
}

func routingScopePlan(t *testing.T, vault *secret.Vault, id int64, choices ...RoutingChoice) RoutingPlan {
	t.Helper()
	raw, err := json.Marshal(RoutingDocument{Choices: choices})
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := vault.Encrypt(fmt.Sprintf("xnode.routes.%d", id), string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return RoutingPlan{ProxyUserID: id, Phase: "active", Cipher: cipher}
}

func TestManagedRoutingSkipsPlansThatDoNotTouchNode(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &routingScopeRepository{plans: []RoutingPlan{
		routingScopePlan(t, vault, 10, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b"}),
		{ProxyUserID: 11, Phase: "disabled"},
	}}
	service := NewService(repo, nil, vault, nil, nil)
	outs, rules, owners, err := service.managedRouting(context.Background(), "node", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 0 || len(rules) != 0 || !owners[10] || !owners[11] || repo.identityReads != 0 {
		t.Fatalf("unrelated plan was expanded: outs=%d rules=%d owners=%v identityReads=%d", len(outs), len(rules), owners, repo.identityReads)
	}
}

func TestManagedRoutingKeepsPlansThatTouchNode(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("identity read")
	repo := &routingScopeRepository{
		plans:       []RoutingPlan{routingScopePlan(t, vault, 12, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "node"})},
		identityErr: failure,
	}
	service := NewService(repo, nil, vault, nil, nil)
	if _, _, _, err = service.managedRouting(context.Background(), "node", nil); !errors.Is(err, failure) || repo.identityReads != 1 {
		t.Fatalf("node-scoped plan was skipped: err=%v reads=%d", err, repo.identityReads)
	}
}
