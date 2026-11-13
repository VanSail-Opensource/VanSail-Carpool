package xnode

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagedRoutingDoesNotPublishAnIncompleteReadAsAnEmptyGraph(t *testing.T) {
	for _, cause := range []error{errors.New("temporary database outage"), context.DeadlineExceeded, ErrUnavailable} {
		t.Run(cause.Error(), func(t *testing.T) {
			service, repo, _ := routingReadFailureFixture(t, cause)
			repo.plans[0] = routingScopePlan(t, service.vault, 7, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b"})
			if _, _, _, err := service.managedRouting(context.Background(), "a", nil); !errors.Is(err, cause) {
				t.Fatal("dependency failure became a successful empty graph", err)
			}
		})
	}
	service, repo, _ := routingReadFailureFixture(t, ErrInvalid)
	repo.plans[0] = routingScopePlan(t, service.vault, 7, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b"})
	outs, rules, _, err := service.managedRouting(context.Background(), "a", nil)
	if err != nil || len(outs) != 0 || len(rules) != 0 {
		t.Fatal("invalid routing was emitted", outs, rules, err)
	}
}

func TestRoutingValidationFailureRetriesWithoutAnUnrelatedVersionChange(t *testing.T) {
	for _, cause := range []error{errors.New("temporary database outage"), ErrUnavailable, ErrInvalid} {
		t.Run(cause.Error(), func(t *testing.T) {
			service, repo, settings := routingReadFailureFixture(t, cause)
			now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
			service.now = func() time.Time { return now }
			repo.plans = []RoutingPlan{
				routingScopePlan(t, service.vault, 7, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b"}),
				routingScopePlan(t, service.vault, 8, RoutingChoice{ID: "cd", InboundNodeID: "c", OutboundNodeID: "d"}),
			}
			err := service.reconcileRoutingPlans(context.Background())
			if !errors.Is(err, cause) || settings.reads != 2 || len(repo.queued) != 0 || repo.advances != 0 {
				t.Fatal("validation failure was lost or blocked another owner", err, settings.reads, repo.queued)
			}
			if finish, err := service.routingReconcileNeeded(context.Background()); err != nil || finish != nil {
				t.Fatal("failed validation did not respect retry backoff", err)
			}
			now = now.Add(2 * time.Second)
			if finish, err := service.routingReconcileNeeded(context.Background()); err != nil || finish == nil {
				t.Fatal("failed validation was cached indefinitely", err)
			}
		})
	}
}
