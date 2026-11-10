package xnode

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"slices"
	"strconv"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) routingPlanNodes(plans ...RoutingPlan) ([]string, error) {
	nodes := []string{}
	for _, p := range plans {
		for _, cipher := range []string{p.Cipher, p.PreviousCipher} {
			if cipher == "" {
				continue
			}
			copy := p
			copy.Cipher = cipher
			doc, err := s.routingDocument(copy)
			if err != nil {
				return nil, err
			}
			for _, n := range doc.LegacyNodes {
				if !slices.Contains(nodes, n) {
					nodes = append(nodes, n)
				}
			}
			for _, r := range doc.Choices {
				for _, n := range []string{r.InboundNodeID, r.OutboundNodeID} {
					if !slices.Contains(nodes, n) {
						nodes = append(nodes, n)
					}
				}
			}
		}
	}
	slices.Sort(nodes)
	return nodes, nil
}
func (s *Service) runRoutingOperation(ctx context.Context, operation providerops.Operation, after ...func(context.Context) error) error {
	if providerops.Terminal(providerops.Status(operation.Receipt.Status)) {
		return nil
	}
	repo, ok := s.routingRepository()
	operations, yes := s.repo.(operationRepository)
	if !ok || !yes || s.master == nil {
		return ErrUnavailable
	}
	items, err := operations.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].TargetType != "xnode_user" {
		return ErrInvalid
	}
	id, err := strconv.ParseInt(items[0].TargetID, 10, 64)
	if err != nil {
		return ErrInvalid
	}
	plan, err := repo.XNodeRoutingPlan(ctx, id)
	if err != nil {
		return err
	}
	if plan.OperationID != operation.Receipt.ID {
		return ErrConflict
	}
	operation, err = operations.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now().UTC())
	if err != nil {
		return err
	}
	nodes, err := s.routingPlanNodes(plan)
	if err != nil {
		return err
	}
	for _, phase := range []string{"revoking", "applying", "enabling"} {
		plan, err = repo.XNodeRoutingPlan(ctx, id)
		if err != nil {
			return err
		}
		if plan.Phase != phase {
			continue
		}
		doc, err := s.routingDocument(plan)
		if err != nil {
			return err
		}
		if phase == "revoking" {
			refresh, err := s.routingRefreshGeneration(plan)
			if err != nil {
				return err
			}
			if refresh {
				// Dependency-only refresh: seal the applied generation's counters and
				// apply the replacement directly. Crossing the revocation barrier
				// would remove the member's rules until every node acknowledges.
				if err = s.captureRoutingCounters(ctx, nodes, id); err != nil {
					return err
				}
				if err = repo.AdvanceXNodeRouting(ctx, id, operation.Receipt.ID, phase, "applying", s.now().UTC()); err != nil {
					return err
				}
				continue
			}
		}
		identity, err := s.repo.XNodeIdentity(ctx, id)
		if err != nil {
			return err
		}
		_, validationErr := s.routingInput(ctx, identity, doc.Choices)
		// Only a graph that cannot be validated at all fails the generation. A
		// digest mismatch is ordinary dependency drift: the applied rules are
		// re-derived from the current inventory, and reconciliation re-prepares the
		// saved generation instead of disabling the member's routing.
		if validationErr != nil {
			if err = repo.AdvanceXNodeRouting(ctx, id, operation.Receipt.ID, phase, "disabled", s.now().UTC()); err != nil {
				return err
			}
			plan.Phase = "disabled"
			if err = s.syncRoutingNodes(ctx, nodes, id, true, plan); err != nil {
				return err
			}
			return s.completeRoutingOperation(ctx, operation, providerops.StatusFailed, "XNODE_ROUTING_DEPENDENCIES_CHANGED")
		}
		if err = s.syncRoutingNodes(ctx, nodes, id, phase == "revoking", plan); err != nil {
			return err
		}
		next := map[string]string{"revoking": "applying", "applying": "enabling", "enabling": "active"}[phase]
		if err = repo.AdvanceXNodeRouting(ctx, id, operation.Receipt.ID, phase, next, s.now().UTC()); err != nil {
			return err
		}
	}
	plan, err = repo.XNodeRoutingPlan(ctx, id)
	if err != nil {
		return err
	}
	if plan.Phase == "disabled" {
		if err = s.syncRoutingNodes(ctx, nodes, id, true, plan); err != nil {
			return err
		}
		return s.completeRoutingOperation(ctx, operation, providerops.StatusFailed, "XNODE_ROUTING_DEPENDENCIES_CHANGED")
	}
	if plan.Phase != "active" {
		return ErrConflict
	}
	if err = s.syncRoutingNodes(ctx, nodes, id, false, plan); err != nil {
		return err
	}
	for _, next := range after {
		if err = next(ctx); err != nil {
			return err
		}
	}
	return s.completeRoutingOperation(ctx, operation, providerops.StatusSucceeded, "")
}
func (s *Service) syncRoutingNodes(ctx context.Context, nodes []string, owner int64, drain bool, plans ...RoutingPlan) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := syncDeploymentNodes(ctx, s.master, nodes); err != nil {
		return err
	}
	for _, plan := range plans {
		if err := s.checkRoutingDeployment(ctx, nodes, plan); err != nil {
			return err
		}
	}
	for _, node := range nodes {
		if drain {
			// Applied revocation fences the old routes and transit credentials.
			// Seal their counters, but leave immutable journal replay to accounting:
			// routing neither settles nor resets the owner's paid period.
			if _, err := s.master.captureTraffic(ctx, node, wire.TrafficFlush{UserID: int(owner)}); err != nil {
				return fmt.Errorf("routing capture node %s: %w", node, err)
			}
		}
	}
	return nil
}
func (s *Service) completeRoutingOperation(ctx context.Context, operation providerops.Operation, status providerops.Status, code string) error {
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	repo := s.repo.(operationRepository)
	_, err := repo.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, providerops.Completion{Status: status, ErrorCode: code, ProviderReference: operation.Receipt.ID, ResultJSON: fmt.Sprintf(`{"routing":true,"operationId":%q}`, operation.Receipt.ID)}, s.now().UTC())
	return err
}
func (s *Service) handleRoutingOperation(operation providerops.Operation, job model.OutboxJob) error {
	return s.master.deferMemberOperation(job, func(ctx context.Context) error { return s.runRoutingOperation(ctx, operation) })
}
