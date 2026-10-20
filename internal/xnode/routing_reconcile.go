package xnode

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	if err := s.migrateLegacyRouting(ctx); err != nil {
		return err
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return err
	}
	for _, plan := range plans {
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
		if plan.Phase != "active" {
			continue
		}
		input, validationErr := s.routingInput(ctx, identity, doc.Choices)
		if validationErr != nil {
			if err = repo.AdvanceXNodeRouting(ctx, plan.ProxyUserID, plan.OperationID, "active", "disabled", s.now().UTC()); err != nil {
				return err
			}
			continue
		}
		if input.dependencies == doc.Dependencies {
			continue
		}
		key := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", plan.ProxyUserID, plan.Revision, input.dependencies)))
		if _, err = s.QueueOwnedRouting(ctx, identity.UserID, fmt.Sprintf("routing-refresh-%x", key[:20]), plan.Revision, doc.Choices); err != nil {
			return err
		}
	}
	return nil
}
