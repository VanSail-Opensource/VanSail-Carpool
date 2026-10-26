package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestCheckoutOperationsDelegatePricingWhileAutomaticRenewalIsEnabled(t *testing.T) {
	t.Parallel()
	repository := &automaticRenewalCatalogRepository{catalogRepository: &catalogRepository{
		combos: []model.Combo{{ID: "combo"}},
		addons: []model.SquadProduct{{ID: "addon", GroupID: "addon"}},
	}, enabled: true}
	service := newAutomaticRenewalCatalogService(repository)
	user := model.User{ID: "member", OnboardingState: "complete"}
	if snapshot, err := service.CatalogForUser(context.Background(), user); err != nil || len(snapshot.Combos) != 1 || len(snapshot.Addons) != 1 {
		t.Fatalf("CatalogForUser() = (%+v, %v), want displayable catalog", snapshot, err)
	}
	if _, err := service.Quote(context.Background(), user, "combo", nil, ""); errors.Is(err, ErrAutoRenewalEnabled) {
		t.Fatalf("Quote() error = %v, blanket renewal lock must not apply", err)
	}
	if _, err := service.Purchase(context.Background(), user, "combo", nil, "purchase-key"); errors.Is(err, ErrAutoRenewalEnabled) {
		t.Fatalf("Purchase() error = %v, blanket renewal lock must not apply", err)
	}
}

func TestAutomaticRenewalToggleDefersBalanceValidationToSettlement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	plan := database.AutoRenewalPlan{Purchase: model.Purchase{ID: "purchase", UserID: "member"}, Combo: model.Combo{ID: "combo", ValidityDays: 30,
		IncludedSquads: []model.SquadProduct{{GroupID: "included"}}}, GrossMinor: 100, NetMinor: 100,
		ScheduledAt: now.Add(30 * 24 * time.Hour), NextCycleEndsAt: now.Add(60 * 24 * time.Hour)}
	repository := &automaticRenewalCatalogRepository{catalogRepository: &catalogRepository{
		combos: []model.Combo{plan.Combo}, balance: model.TXBMoney(99),
	}, plan: plan}
	service := newAutomaticRenewalCatalogService(repository)
	service.now = func() time.Time { return now }
	user := model.User{ID: "member"}
	status, err := service.AutomaticRenewal(context.Background(), user, plan.Purchase.ID)
	if err != nil || !status.CanEnable || status.IneligibleReason != nil {
		t.Fatalf("AutomaticRenewal() = (%+v, %v)", status, err)
	}
	updated, err := service.SetAutomaticRenewal(context.Background(), user, plan.Purchase.ID, true)
	if err != nil || !updated.Enabled || !repository.setEnabled {
		t.Fatalf("SetAutomaticRenewal(eligible) = (%+v, %v), stored=%t", updated, err, repository.setEnabled)
	}
}

type automaticRenewalCatalogRepository struct {
	*catalogRepository
	plan       database.AutoRenewalPlan
	enabled    bool
	setEnabled bool
}

func (r *automaticRenewalCatalogRepository) AutoRenewalPlan(context.Context, string, string, time.Time) (database.AutoRenewalPlan, error) {
	return r.plan, nil
}
func (r *automaticRenewalCatalogRepository) SetAutoRenewal(_ context.Context, _ string, _ string, enabled bool, _ time.Time) error {
	r.setEnabled = enabled
	return nil
}
func (r *automaticRenewalCatalogRepository) DueAutoRenewals(context.Context, time.Time) ([]database.DueAutoRenewal, error) {
	return nil, nil
}
func (r *automaticRenewalCatalogRepository) CommitAutoRenewal(context.Context, string, time.Time) (model.Purchase, error) {
	return model.Purchase{}, nil
}

func (r *automaticRenewalCatalogRepository) AutoRenewalPlanExcludingAddons(_ context.Context, _ string, _ string, excluded []string, _ time.Time) (database.AutoRenewalPlan, error) {
	blocked := make(map[string]struct{}, len(excluded))
	for _, id := range excluded {
		blocked[id] = struct{}{}
	}
	plan := r.plan
	plan.Addons = make([]model.SquadProduct, 0, len(r.plan.Addons))
	for _, addon := range r.plan.Addons {
		if _, skip := blocked[addon.GroupID]; skip {
			plan.GrossMinor -= addon.PriceTXBMinor
			plan.NetMinor -= addon.PriceTXBMinor
			continue
		}
		plan.Addons = append(plan.Addons, addon)
	}
	return plan, nil
}

func (r *automaticRenewalCatalogRepository) CommitAutoRenewalExcludingAddons(context.Context, string, []string, time.Time) (model.Purchase, error) {
	return model.Purchase{}, nil
}
func (r *automaticRenewalCatalogRepository) MarkAutoRenewalFailed(context.Context, string, string, time.Time) error {
	return nil
}
func (r *automaticRenewalCatalogRepository) HasEnabledAutoRenewal(context.Context, string, time.Time) (bool, error) {
	return r.enabled, nil
}

func newAutomaticRenewalCatalogService(repository *automaticRenewalCatalogRepository) *Service {
	return newCatalogServiceForTest(repository, &catalogPurchaseRemote{catalogProxy: &catalogProxy{}})
}
