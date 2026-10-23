package xnode

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// prepareOwnedRouting reuses validation/credentials/preflight for both routing
// API compatibility and atomic DIY saves. Preparation only reads state; the
// repository fences revisions/replay when publishing the resulting plan.
func (s *Service) prepareOwnedRouting(ctx context.Context, identity Identity, revision int64, choices []RoutingChoice) (RoutingPlan, error) {
	repo, ok := s.routingRepository()
	if !ok {
		return RoutingPlan{}, ErrUnavailable
	}
	old, err := repo.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return RoutingPlan{}, err
	}
	// The database rechecks revisions after operation replay detection, so an
	// identical request remains replayable after successful deployment.
	in, err := s.routingInput(ctx, identity, choices)
	if err != nil {
		return RoutingPlan{}, err
	}
	choices = in.choices
	// Persist endpoint IDs only on a member save. Existing node-only documents
	// retain their dependency digest and runtime behavior until that save.
	in, err = s.routingInput(ctx, identity, choices)
	if err != nil {
		return RoutingPlan{}, err
	}
	seed, err := ids.Token(32)
	if err != nil {
		return RoutingPlan{}, err
	}
	doc := RoutingDocument{Choices: choices, Seed: seed, Dependencies: in.dependencies}
	plan := RoutingPlan{ProxyUserID: identity.ProxyUserID, Revision: revision, Phase: "active", LegacyCipher: old.LegacyCipher}
	if plan.LegacyCipher == "" {
		plan.LegacyCipher, err = s.archiveLegacyRouting(ctx, identity)
		if err != nil {
			return RoutingPlan{}, err
		}
	}
	var archived []struct {
		NodeID string `json:"nodeId"`
	}
	if err = s.open(fmt.Sprintf("xnode.routes.legacy.%d", identity.ProxyUserID), plan.LegacyCipher, &archived); err != nil {
		return RoutingPlan{}, err
	}
	for _, a := range archived {
		doc.LegacyNodes = append(doc.LegacyNodes, a.NodeID)
	}
	plan.Cipher, err = s.seal(fmt.Sprintf("xnode.routes.%d", identity.ProxyUserID), doc)
	if err != nil {
		return RoutingPlan{}, err
	}
	if err = s.preflightRouting(ctx, plan, old); err != nil {
		return RoutingPlan{}, err
	}
	return plan, nil
}
