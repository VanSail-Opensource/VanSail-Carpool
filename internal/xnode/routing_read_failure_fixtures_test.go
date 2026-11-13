package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type routingReadFailureSettings struct {
	Settings
	failure error
	reads   int
}

func (s *routingReadFailureSettings) Optional(context.Context, string) (string, error) {
	s.reads++
	return "", s.failure
}

type routingReadFailureRepository struct {
	Repository
	routingRepository
	operationRepository
	plans     []RoutingPlan
	legacy    []Customization
	queued    []RoutingPlan
	advances  int
	completed []providerops.Completion
}

func (r *routingReadFailureRepository) XNodeIdentity(_ context.Context, id int64) (Identity, error) {
	return Identity{ProxyUserID: id, UserID: "member"}, nil
}
func (r *routingReadFailureRepository) XNodeNodes(context.Context) ([]Node, error) {
	return nil, nil
}
func (r *routingReadFailureRepository) XNodeUnmanagedRoutes(context.Context, time.Time) ([]Customization, error) {
	return r.legacy, nil
}
func (r *routingReadFailureRepository) XNodeRoutingPlans(context.Context) ([]RoutingPlan, error) {
	return r.plans, nil
}
func (r *routingReadFailureRepository) XNodeRoutingPlan(_ context.Context, id int64) (RoutingPlan, error) {
	for _, plan := range r.plans {
		if plan.ProxyUserID == id {
			return plan, nil
		}
	}
	return RoutingPlan{}, ErrNotFound
}
func (r *routingReadFailureRepository) QueueXNodeRouting(_ context.Context, plan RoutingPlan, _ providerops.CreateInput, _ time.Time) (model.OperationReceipt, error) {
	r.queued = append(r.queued, plan)
	return model.OperationReceipt{ID: "migration"}, nil
}
func (r *routingReadFailureRepository) AdvanceXNodeRouting(_ context.Context, id int64, operation, from, to string, _ time.Time) error {
	for i := range r.plans {
		if r.plans[i].ProxyUserID == id && r.plans[i].OperationID == operation && r.plans[i].Phase == from {
			r.advances++
			r.plans[i].Phase = to
			return nil
		}
	}
	return ErrConflict
}
func (r *routingReadFailureRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{{Key: "routing", TargetType: "xnode_user", TargetID: "7"}}, nil
}
func (r *routingReadFailureRepository) BeginProviderOperationAttempt(_ context.Context, id string, _ time.Time) (providerops.Operation, error) {
	return providerops.Operation{Receipt: model.OperationReceipt{ID: id, Status: "processing"}, Attempts: 1}, nil
}
func (r *routingReadFailureRepository) CompleteXNodeProviderOperation(_ context.Context, _ string, _ int, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.completed = append(r.completed, completion)
	return providerops.Operation{}, nil
}
func (r *routingReadFailureRepository) WakeXNodeRouting(context.Context, time.Time) error {
	return nil
}
func (r *routingReadFailureRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return 7, time.Time{}, nil
}

func routingReadFailureFixture(t *testing.T, failure error) (*Service, *routingReadFailureRepository, *routingReadFailureSettings) {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	plan := routingScopePlan(t, vault, 7)
	plan.Phase, plan.OperationID = "applying", "operation"
	repo := &routingReadFailureRepository{plans: []RoutingPlan{plan}}
	settings := &routingReadFailureSettings{failure: failure}
	service := NewService(repo, settings, vault, nil, nil)
	service.master = &Master{}
	return service, repo, settings
}
