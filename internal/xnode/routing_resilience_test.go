package xnode

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type renewalRepository struct {
	Repository
	desired   *model.Purchase
	purchases []model.Purchase
}

func (r *renewalRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return r.desired, nil
}
func (r *renewalRepository) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	return r.purchases, nil
}
func (r *renewalRepository) ContinuityEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return nil, nil
}

func TestRoutingKeepsPurchaseRenewalBoundaries(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name      string
		desired   *model.Purchase
		purchases []model.Purchase
		renewing  bool
	}{
		{name: "effective purchase", desired: &model.Purchase{Status: "active"}, renewing: false},
		{name: "started queued successor", purchases: []model.Purchase{{Status: "queued", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour)}}, renewing: true},
		{name: "prepaid future successor", purchases: []model.Purchase{{Status: "queued", ValidFrom: now.Add(time.Hour), ValidUntil: now.Add(2 * time.Hour)}}, renewing: true},
		{name: "activation in flight", purchases: []model.Purchase{{Status: "activating", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour)}}, renewing: true},
		{name: "auto renewal settling", purchases: []model.Purchase{{Status: "active", AutoRenewEnabled: true, ValidUntil: now.Add(-time.Minute)}}, renewing: true},
		{name: "auto renewal deferred settlement", purchases: []model.Purchase{{Status: "active", AutoRenewEnabled: true, ValidUntil: now.Add(-72 * time.Hour)}}, renewing: true},
		{name: "auto renewal failed preference", purchases: []model.Purchase{{Status: "active", ValidUntil: now.Add(-72 * time.Hour)}}, renewing: false},
		{name: "manual expiry", purchases: []model.Purchase{{Status: "active", ValidUntil: now.Add(-time.Minute)}}, renewing: false},
		{name: "no purchase", renewing: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &renewalRepository{desired: test.desired, purchases: test.purchases}
			service := NewService(repo, nil, nil, nil, nil)
			renewing, err := service.routingPurchaseRenewing(context.Background(), "owner")
			if err != nil || renewing != test.renewing {
				t.Fatalf("renewing=%v want %v: %v", renewing, test.renewing, err)
			}
		})
	}
}

func TestRoutingDependencyRefreshKeepsAppliedGraph(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, nil, vault, nil, nil)
	saved := []RoutingChoice{{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "google"}}
	if !sameRoutingGraph(saved, []RoutingChoice{{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "google", InboundEndpointID: "pin-a", OutboundEndpointID: "pin-b"}}) {
		t.Fatal("endpoint pins changed the saved graph")
	}
	if sameRoutingGraph(saved, []RoutingChoice{{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "ai"}}) {
		t.Fatal("template change was treated as a refresh")
	}
	plan := routingScopePlan(t, vault, 10, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "google"})
	previous := routingScopePlan(t, vault, 10, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "google", InboundEndpointID: "pin-a", OutboundEndpointID: "pin-b"})
	plan.PreviousCipher = previous.Cipher
	refresh, err := service.routingRefreshGeneration(plan)
	if err != nil || !refresh {
		t.Fatalf("dependency-only refresh not detected: %v", err)
	}
	changed := routingScopePlan(t, vault, 10, RoutingChoice{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b", TemplateID: "ai"})
	plan.PreviousCipher = changed.Cipher
	if refresh, err = service.routingRefreshGeneration(plan); err != nil || refresh {
		t.Fatalf("graph edit detected as refresh: %v", err)
	}
	plan.PreviousCipher = ""
	if refresh, err = service.routingRefreshGeneration(plan); err != nil || refresh {
		t.Fatalf("first generation detected as refresh: %v", err)
	}
}

func TestRoutingReconcileRefreshesInsteadOfDisabling(t *testing.T) {
	choices := []RoutingChoice{{ID: "ab", InboundNodeID: "a", OutboundNodeID: "b"}}
	doc := RoutingDocument{Choices: choices, Dependencies: "saved"}
	for _, test := range []struct {
		name       string
		plan       RoutingPlan
		doc        RoutingDocument
		digest     string
		validation error
		refresh    bool
	}{
		{name: "active settled", plan: RoutingPlan{Phase: "active"}, doc: doc, digest: "saved", refresh: false},
		{name: "active drift", plan: RoutingPlan{Phase: "active"}, doc: doc, digest: "new", refresh: true},
		{name: "disabled dependency drift", plan: RoutingPlan{Phase: "disabled", Reason: "routing_dependencies_changed"}, doc: doc, digest: "new", refresh: true},
		{name: "access review stays disabled", plan: RoutingPlan{Phase: "disabled", Reason: "routing_access_removed"}, doc: doc, digest: "new", refresh: false},
		{name: "disabled empty graph", plan: RoutingPlan{Phase: "disabled", Reason: "routing_legacy_incompatible"}, doc: RoutingDocument{}, digest: "new", refresh: false},
		{name: "node removal stays terminal", plan: RoutingPlan{Phase: "disabled", Reason: "routing_node_removed"}, doc: doc, digest: "new", refresh: false},
		{name: "transient validation gap", plan: RoutingPlan{Phase: "active"}, doc: doc, digest: "saved", validation: ErrInvalid, refresh: false},
		{name: "in-flight generation", plan: RoutingPlan{Phase: "revoking"}, doc: doc, digest: "new", refresh: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := routingRefreshDue(test.plan, test.doc, test.digest, test.validation); got != test.refresh {
				t.Fatalf("refresh=%v want %v", got, test.refresh)
			}
		})
	}
}

func TestRoutingRefreshContinuesAfterMemberFailure(t *testing.T) {
	requests := []routingRefreshRequest{
		{plan: RoutingPlan{ProxyUserID: 1, Phase: "active"}},
		{plan: RoutingPlan{ProxyUserID: 2, Phase: "active"}},
	}
	attempted := []int64{}
	failure := errors.New("preflight unavailable")
	deferred := routingRefreshAll(requests, func(request routingRefreshRequest) error {
		attempted = append(attempted, request.plan.ProxyUserID)
		if request.plan.ProxyUserID == 1 {
			return failure
		}
		return nil
	})
	if len(attempted) != 2 || attempted[1] != 2 {
		t.Fatalf("later member was blocked: %v", attempted)
	}
	if !errors.Is(deferred, failure) || !strings.Contains(deferred.Error(), "routing refresh for 1") {
		t.Fatalf("failure was swallowed: %v", deferred)
	}
	if err := routingRefreshAll(requests[:1], func(routingRefreshRequest) error { return ErrConflict }); err != nil {
		t.Fatalf("revision race became deferred work: %v", err)
	}
}
