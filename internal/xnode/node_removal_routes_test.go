package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"slices"
	"testing"
)

func TestNodeRemovalPrunesChoicesAndKeepsAllRevocationParticipants(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	plan := routingScopePlan(t, vault, 10, RoutingChoice{ID: "removed", InboundNodeID: "a", OutboundNodeID: "b"}, RoutingChoice{ID: "kept", InboundNodeID: "a", OutboundNodeID: "c"})
	old := routingScopePlan(t, vault, 10, RoutingChoice{ID: "previous", InboundNodeID: "b", OutboundNodeID: "d"})
	plan.PreviousCipher, plan.LegacyCipher = old.Cipher, "immutable-evidence"
	other := routingScopePlan(t, vault, 11, RoutingChoice{ID: "unrelated", InboundNodeID: "c", OutboundNodeID: "d"})
	repo := &routingScopeRepository{plans: []RoutingPlan{plan, other}}
	service := NewService(repo, nil, vault, nil, nil)
	changes, err := service.prepareRemovalRoutes(context.Background(), "b")
	if err != nil || len(changes) != 1 {
		t.Fatalf("changes=%+v %v", changes, err)
	}
	changed := changes[0]
	doc, err := service.routingDocument(changed)
	if err != nil || len(doc.Choices) != 1 || doc.Choices[0].ID != "kept" || slices.Contains(doc.LegacyNodes, "b") || changed.LegacyCipher != plan.LegacyCipher {
		t.Fatalf("incorrect prune: %+v %+v %v", changed, doc, err)
	}
	proof := changed
	proof.Cipher = changed.PreviousCipher
	previous, err := service.routingDocument(proof)
	if err != nil || len(previous.Choices) != 3 || !slices.Contains(previous.LegacyNodes, "b") || !slices.Contains(previous.LegacyNodes, "d") {
		t.Fatalf("revocation evidence lost: %+v %v", previous, err)
	}
	changed.Phase, changed.Reason = "disabled", routingRemovalPending+"b"
	repo.plans = []RoutingPlan{changed}
	if replay, err := service.prepareRemovalRoutes(context.Background(), "b"); err != nil || len(replay) != 0 {
		t.Fatalf("replayed prune: %+v %v", replay, err)
	}
	if _, err := service.prepareRemovalRoutes(context.Background(), "a"); !errors.Is(err, ErrConflict) {
		t.Fatalf("concurrent removal replaced pending proof: %v", err)
	}
}
