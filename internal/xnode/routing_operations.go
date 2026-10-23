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
		identity, err := s.repo.XNodeIdentity(ctx, id)
		if err != nil {
			return err
		}
		input, validationErr := s.routingInput(ctx, identity, doc.Choices)
		if validationErr != nil || input.dependencies != doc.Dependencies {
			if err = repo.AdvanceXNodeRouting(ctx, id, operation.Receipt.ID, phase, "disabled", s.now().UTC()); err != nil {
				return err
			}
			if err = s.syncRoutingNodes(ctx, nodes, id, true); err != nil {
				return err
			}
			return s.completeRoutingOperation(ctx, operation, providerops.StatusFailed, "XNODE_ROUTING_DEPENDENCIES_CHANGED")
		}
		if err = s.syncRoutingNodes(ctx, nodes, id, phase == "revoking"); err != nil {
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
		if err = s.syncRoutingNodes(ctx, nodes, id, true); err != nil {
			return err
		}
		return s.completeRoutingOperation(ctx, operation, providerops.StatusFailed, "XNODE_ROUTING_DEPENDENCIES_CHANGED")
	}
	if plan.Phase != "active" {
		return ErrConflict
	}
	for _, next := range after {
		if err = next(ctx); err != nil {
			return err
		}
	}
	return s.completeRoutingOperation(ctx, operation, providerops.StatusSucceeded, "")
}
func (s *Service) syncRoutingNodes(ctx context.Context, nodes []string, owner int64, drain bool) error {
	for _, node := range nodes {
		if err := s.master.Sync(ctx, node); err != nil {
			return err
		}
		if err := s.master.WaitApplied(ctx, node); err != nil {
			return err
		}
		if drain {
			// Applied revocation fences the old routes and transit credentials.
			// Seal their counters, but leave immutable journal replay to accounting:
			// routing neither settles nor resets the owner's paid period.
			if _, err := s.master.captureTraffic(ctx, node, wire.TrafficFlush{UserID: int(owner)}); err != nil {
				return err
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
	return s.master.deferOperation(job, 15*time.Minute, func(ctx context.Context) error { return s.runRoutingOperation(ctx, operation) })
}
