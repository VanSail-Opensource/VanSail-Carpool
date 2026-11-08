package xnode

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
)

// Called only by the existing master reconciler, outside the socket reader.
// An invalid dependency removes the whole graph; a valid change receives a new
// durable generation and passes the same revoke/apply/enable barriers.
func (s *Service) reconcileRoutingPlans(ctx context.Context) (result error) {
	repo, ok := s.routingRepository()
	if !ok {
		return nil
	}
	finish, err := s.routingReconcileNeeded(ctx)
	if err != nil {
		return err
	}
	if finish == nil {
		return nil
	}
	defer func() { finish(result == nil) }()
	// A dependency change is new recovery evidence for routing and DDNS desired
	// state, alongside startup/reconnect. No periodic competing scheduler is added.
	if err := repo.WakeXNodeRouting(ctx, s.now()); err != nil {
		return err
	}
	if err := s.migrateLegacyRouting(ctx); err != nil {
		return err
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if strings.HasPrefix(plan.Reason, routingRemovalPending) {
			continue
		}
		identity, err := s.repo.XNodeIdentity(ctx, plan.ProxyUserID)
		if err != nil {
			return err
		}
		doc, err := s.routingDocument(plan)
		if err != nil {
			return err
		}
		if pruned, err := s.pruneRoutingAccess(ctx, plan, identity, doc); err != nil {
			return err
		} else if pruned {
			continue
		}
		// A committed full access prune is already the desired disabled state.
		// Regenerating its empty graph would replace revocation evidence with a
		// new operation. Partial prunes and recoverable failures still reconcile.
		if plan.Phase == "disabled" && (plan.Reason == "routing_access_removed" || plan.Reason == "routing_node_removed") && len(doc.Choices) == 0 {
			continue
		}
		if plan.Phase != "active" && plan.Phase != "disabled" {
			continue
		}
		input, validationErr := s.routingInput(ctx, identity, doc.Choices)
		if validationErr != nil {
			if plan.Phase == "active" {
				if err = repo.AdvanceXNodeRouting(ctx, plan.ProxyUserID, plan.OperationID, "active", "disabled", s.now().UTC()); err != nil {
					return err
				}
			}
			continue
		}
		if plan.Phase == "active" && input.dependencies == doc.Dependencies {
			continue
		}
		if !stableRoutingChoices(doc.Choices) && input.dependencies != doc.Dependencies {
			if plan.Phase == "active" {
				if err = repo.AdvanceXNodeRouting(ctx, plan.ProxyUserID, plan.OperationID, "active", "disabled", s.now().UTC()); err != nil {
					return err
				}
			}
			continue
		}
		key := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", plan.ProxyUserID, plan.Revision, input.dependencies)))
		if _, err = s.QueueOwnedRouting(ctx, identity.UserID, fmt.Sprintf("routing-refresh-%x", key[:20]), plan.Revision, doc.Choices); err != nil {
			return err
		}
	}
	return nil
}
