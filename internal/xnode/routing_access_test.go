package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type prunedRoutingRepository struct {
	*routingScopeRepository
	version int64
	wakes   int
}

func (r *prunedRoutingRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return r.version, time.Time{}, nil
}
func (r *prunedRoutingRepository) WakeXNodeRouting(context.Context, time.Time) error {
	r.wakes++
	return nil
}
func (r *prunedRoutingRepository) XNodeUnmanagedRoutes(context.Context, time.Time) ([]Customization, error) {
	return nil, nil
}
func (r *prunedRoutingRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	return Identity{ProxyUserID: 10, UserID: "owner"}, nil
}
func (r *prunedRoutingRepository) AdvanceXNodeRouting(context.Context, int64, string, string, string, time.Time) error {
	return nil
}

type prunedRoutingSettings struct{ reads int }

func (s *prunedRoutingSettings) Optional(context.Context, string) (string, error) {
	s.reads++
	return "", ErrUnavailable
}
func (*prunedRoutingSettings) PutXNodeGlobalAtVersion(context.Context, string, string, int64, time.Time) error {
	return nil
}

func TestRoutingReconciliationRetainsCommittedFullAccessPrune(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		name, phase, reason string
		choices             []RoutingChoice
		inputReads          int
	}{
		{name: "fully-pruned", phase: "disabled", reason: "routing_access_removed"},
		{name: "empty-node-removal", phase: "disabled", reason: "routing_node_removed"},
		{name: "pending-node-removal", phase: "disabled", reason: routingRemovalPending + "node"},
		{name: "dependency-recovery", phase: "disabled", reason: "routing_dependencies_changed", inputReads: 3},
		{name: "active-empty", phase: "active", reason: "routing_access_removed", inputReads: 3},
		{name: "partially-pruned", phase: "disabled", reason: "routing_access_removed", choices: []RoutingChoice{{ID: "keep"}}, inputReads: 3},
	} {
		t.Run(step.name, func(t *testing.T) {
			plan := routingScopePlan(t, vault, 10, step.choices...)
			plan.Phase, plan.Reason = step.phase, step.reason
			repo := &prunedRoutingRepository{routingScopeRepository: &routingScopeRepository{plans: []RoutingPlan{plan}}}
			settings := &prunedRoutingSettings{}
			service := NewService(repo, settings, vault, nil, nil)
			for range 3 {
				// Recheck after edits/startup rather than relying on a cached skip.
				repo.version++
				if err := service.reconcileRoutingPlans(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if settings.reads != step.inputReads || repo.wakes != 3 {
				t.Fatal("full prune regenerated or recovery skipped", settings.reads, repo.wakes)
			}
		})
	}
}

func TestRoutingAccessPrunesLostNodeAndEndpointReferences(t *testing.T) {
	a := Endpoint{ID: "a", NodeID: "a"}
	b := Endpoint{ID: "b", NodeID: "b"}
	c := Endpoint{ID: "c", NodeID: "c"}
	in := routingInput{endpoints: map[string]Endpoint{"a": a, "b": b}, byEndpoint: map[string]Endpoint{"a": a, "b": b}}
	choices := []RoutingChoice{
		{ID: "keep", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "a", OutboundEndpointID: "b"},
		{ID: "lost-node", InboundNodeID: "b", OutboundNodeID: "c", InboundEndpointID: "b", OutboundEndpointID: "c"},
		{ID: "lost-host", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "retired-a", OutboundEndpointID: "b"},
		{ID: "legacy-default", InboundNodeID: "a", OutboundNodeID: "b"},
	}
	kept := accessibleRoutingChoices(choices, in, false)
	if len(kept) != 2 || kept[0].ID != "keep" || kept[1].ID != "legacy-default" {
		t.Fatal(kept)
	}
	in.endpoints["c"], in.byEndpoint["c"] = c, c
	if len(accessibleRoutingChoices(choices, in, false)) != 3 {
		t.Fatal("authorized node removed")
	}
	// While the effective purchase is renewing, combo and legacy choices stay:
	// their inventory is derived from the unsettled term. PAYG/hourly references
	// remain authoritative because their own accounts/enrollments feed them.
	in = routingInput{endpoints: map[string]Endpoint{"a": a, "b": b}, byEndpoint: map[string]Endpoint{"a": a, "b": b}}
	modes := []RoutingChoice{
		{ID: "combo", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "a", OutboundEndpointID: "b"},
		{ID: "legacy", InboundNodeID: "a", OutboundNodeID: "b"},
		{ID: "payg", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "payg-lost", OutboundEndpointID: "b"},
		{ID: "hourly", InboundNodeID: "a", OutboundNodeID: "b", InboundEndpointID: "a", OutboundEndpointID: "hourly-lost"},
	}
	renewing := accessibleRoutingChoices(modes, in, true)
	if len(renewing) != 2 || renewing[0].ID != "combo" || renewing[1].ID != "legacy" {
		t.Fatal(renewing)
	}
}
