package xnode

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
)

// Called only by the existing master reconciler, outside the socket reader.
// Dependency drift is re-prepared as a new durable generation; it never disables
// a member's routing. Only durable access loss (a committed access prune) or
// node removal disables a plan. Unprovable dependencies are retried on the next
// reconcile so transient configuration, asset or inventory gaps recover.
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
	requests := []routingRefreshRequest{}
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
		if identity.UserID == "" {
			// Standalone plans have no member to authorize a re-prepared generation;
			// the compiler keeps applying their saved choices in place.
			continue
		}
		input, validationErr := s.routingInput(ctx, identity, doc.Choices)
		if !routingRefreshDue(plan, doc, input.dependencies, validationErr) {
			continue
		}
		requests = append(requests, routingRefreshRequest{plan: plan, identity: identity, document: doc, dependencies: input.dependencies})
	}
	return routingRefreshAll(requests, func(request routingRefreshRequest) error {
		key := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", request.plan.ProxyUserID, request.plan.Revision, request.dependencies)))
		_, err := s.QueueOwnedRouting(ctx, request.identity.UserID, fmt.Sprintf("routing-refresh-%x", key[:20]), request.plan.Revision, request.document.Choices)
		return err
	})
}

// routingRefreshRequest is one member generation that reconcile must re-prepare.
type routingRefreshRequest struct {
	plan         RoutingPlan
	identity     Identity
	document     RoutingDocument
	dependencies string
}

// routingRefreshAll queues each refreshed generation independently. One member's
// transient failure (offline node preflight, missing capability, revision race)
// must not block every other member's recovery; the joined error still lets the
// caller apply retry backoff. A conflict means another generation already owns
// that revision, so there is nothing to defer.
func routingRefreshAll(requests []routingRefreshRequest, refresh func(routingRefreshRequest) error) (deferred error) {
	for _, request := range requests {
		if err := refresh(request); err != nil {
			if errors.Is(err, ErrConflict) {
				continue
			}
			deferred = errors.Join(deferred, fmt.Errorf("routing refresh for %d: %w", request.plan.ProxyUserID, err))
		}
	}
	return deferred
}

// routingRefreshDue decides whether reconcile should queue a refreshed
// generation. It never disables: a plan whose choices cannot be resolved right
// now is retried after the transient gap. Only dependency drift is recoverable;
// a partial access prune keeps its surviving choices disabled for review, and
// empty graphs or node-removal evidence stay terminal so a fully pruned or
// removed plan is never resurrected.
func routingRefreshDue(plan RoutingPlan, doc RoutingDocument, dependencies string, validationErr error) bool {
	if validationErr != nil || len(doc.Choices) == 0 || plan.Reason == "routing_node_removed" {
		return false
	}
	if plan.Phase == "active" {
		return dependencies != doc.Dependencies
	}
	return plan.Phase == "disabled" && plan.Reason == "routing_dependencies_changed"
}
