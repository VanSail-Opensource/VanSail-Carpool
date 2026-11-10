package xnode

import (
	"context"
	"fmt"
	"slices"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// routingGraphKey identifies the member intent of a choice. Endpoint pins are
// deliberately excluded: resolving the current default endpoint for the same
// node pair does not change the routing graph the member saved.
func routingGraphKey(choice RoutingChoice) string {
	return choice.ID + "\x1f" + choice.InboundNodeID + "\x1f" + choice.OutboundNodeID + "\x1f" + choice.TemplateID
}

// sameRoutingGraph reports whether two generations describe the same saved
// routing graph, regardless of endpoint pins or dependency digest.
func sameRoutingGraph(left, right []RoutingChoice) bool {
	if len(left) != len(right) {
		return false
	}
	a, b := make([]string, 0, len(left)), make([]string, 0, len(right))
	for index := range left {
		a = append(a, routingGraphKey(left[index]))
		b = append(b, routingGraphKey(right[index]))
	}
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// routingRefreshGeneration reports whether the current generation is a
// dependency-only refresh of the previously applied graph. Such generations
// must not cross the revocation barrier: the member's applied routing and its
// sealed counters stay in place until the replacement generation is applied.
func (s *Service) routingRefreshGeneration(plan RoutingPlan) (bool, error) {
	if plan.PreviousCipher == "" {
		return false, nil
	}
	current, err := s.routingDocument(plan)
	if err != nil {
		return false, err
	}
	previous := plan
	previous.Cipher = plan.PreviousCipher
	prior, err := s.routingDocument(previous)
	if err != nil {
		return false, err
	}
	return sameRoutingGraph(current.Choices, prior.Choices), nil
}

// captureRoutingCounters seals the owner's reported counters before a refreshed
// generation installs new transit credentials. Unlike revocation it does not
// sync or remove the applied rules, so routing never disappears for a
// dependency-only refresh.
func (s *Service) captureRoutingCounters(ctx context.Context, nodes []string, owner int64) error {
	if s.master == nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for _, node := range nodes {
		if _, err := s.master.captureTraffic(ctx, node, wire.TrafficFlush{UserID: int(owner)}); err != nil {
			return fmt.Errorf("routing capture node %s: %w", node, err)
		}
	}
	return nil
}
