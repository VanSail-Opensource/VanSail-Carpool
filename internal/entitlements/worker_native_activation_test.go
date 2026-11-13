package entitlements

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type nativeActivationRepository struct {
	*entitlementRepository
	refresh       model.Purchase
	refreshErr    error
	reads         int
	completions   []bool
	expected      []model.Purchase
	generations   []int64
	completionErr error
}

func (r *nativeActivationRepository) PurchaseByID(context.Context, string) (model.Purchase, error) {
	r.reads++
	if r.reads == 1 {
		return r.purchase, r.purchaseErr
	}
	return r.refresh, r.refreshErr
}
func (r *nativeActivationRepository) MarkPurchaseSyncResultIfUnchanged(_ context.Context, purchase model.Purchase, generation int64, success bool, _ time.Time) error {
	r.completions = append(r.completions, success)
	r.expected = append(r.expected, purchase)
	r.generations = append(r.generations, generation)
	return r.completionErr
}

type nativeActivationProvider struct {
	entitlementProxy
	user     model.User
	purchase model.Purchase
	calls    int
	err      error
}

func (r *nativeActivationProvider) ApplyMemberPurchase(_ context.Context, user model.User, purchase model.Purchase) error {
	r.calls++
	r.user, r.purchase = user, purchase
	return r.err
}

func nativeActivationFixture() (*nativeActivationRepository, *nativeActivationProvider, model.OutboxJob) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	purchase := model.Purchase{ID: "purchase", UserID: "owner", Status: "activating", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), UpdatedAt: now}
	repository := &nativeActivationRepository{entitlementRepository: &entitlementRepository{purchase: purchase, user: model.User{ID: "owner", AccountStatus: "active", AccountGeneration: 2}, resetPhaseErr: errors.New("legacy phase must not be read")}, refresh: purchase}
	return repository, &nativeActivationProvider{}, model.OutboxJob{Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase"}`, Attempts: 10}
}

func TestNativeActivationUsesFreshOwnedTermWithoutLegacyReset(t *testing.T) {
	t.Parallel()
	repository, provider, job := nativeActivationFixture()
	repository.refresh.ValidUntil = repository.refresh.ValidUntil.Add(time.Hour)
	repository.refresh.UpdatedAt = repository.refresh.UpdatedAt.Add(time.Minute)
	repository.refresh.TrafficLimitBytes = 321
	worker := newEntitlementWorkerForTest(repository, provider)
	if err := worker.HandleOutbox(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || provider.user.ID != "owner" || provider.purchase.TrafficLimitBytes != 321 || !provider.purchase.UpdatedAt.Equal(repository.refresh.UpdatedAt) ||
		len(repository.completions) != 1 || !repository.completions[0] || repository.generations[0] != 2 || !repository.expected[0].UpdatedAt.Equal(repository.refresh.UpdatedAt) {
		t.Fatal("activation was not bound to refreshed target", provider, repository)
	}
	if provider.removeCalls != 0 || provider.resetCalls != 0 || provider.applyCalls != 0 || repository.markedSuccess != nil {
		t.Fatal("native activation used legacy account-wide mutation or completion")
	}
}

func TestNativeActivationLocalFailuresDoNotMarkProviderFailure(t *testing.T) {
	for _, scenario := range []string{"refresh storage", "owner changed", "cancelled", "completion storage"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			repository, provider, job := nativeActivationFixture()
			failure := errors.New("storage unavailable")
			switch scenario {
			case "refresh storage":
				repository.refreshErr = failure
			case "owner changed":
				repository.refresh.UserID = "other"
			case "cancelled":
				repository.refresh.Status = "cancelled"
			case "completion storage":
				repository.completionErr = failure
			}
			err := newEntitlementWorkerForTest(repository, provider).HandleOutbox(context.Background(), job)
			if (err == nil) != (scenario == "cancelled") {
				t.Fatal("unexpected recovery outcome", err)
			}
			if scenario == "refresh storage" || scenario == "completion storage" {
				if !errors.Is(err, failure) {
					t.Fatal("lost storage cause", err)
				}
			}
			wantCalls := 0
			if scenario == "completion storage" {
				wantCalls = 1
			}
			if provider.calls != wantCalls || len(repository.completions) != wantCalls || repository.markedSuccess != nil {
				t.Fatal("local failure mutated receipt", provider.calls, repository.completions)
			}
			for _, success := range repository.completions {
				if !success {
					t.Fatal("local failure relabeled paid activation failed")
				}
			}
		})
	}
}

func TestNativeActivationTerminalFailureUsesExactComparedSnapshot(t *testing.T) {
	t.Parallel()
	repository, provider, job := nativeActivationFixture()
	provider.err = errors.New("native application failed")
	repository.completionErr = errors.New("purchase was replaced")
	err := newEntitlementWorkerForTest(repository, provider).HandleOutbox(context.Background(), job)
	if !errors.Is(err, provider.err) || !errors.Is(err, repository.completionErr) || len(repository.completions) != 1 || repository.completions[0] ||
		repository.expected[0].ID != "purchase" || repository.generations[0] != 2 || !repository.expected[0].UpdatedAt.Equal(repository.refresh.UpdatedAt) || repository.markedSuccess != nil {
		t.Fatal("failure escaped target compare", err, repository)
	}
}

func TestNativeActivationRequiresConditionalCompletionAdapter(t *testing.T) {
	repository, provider, job := nativeActivationFixture()
	if err := newEntitlementWorkerForTest(repository.entitlementRepository, provider).HandleOutbox(context.Background(), job); err == nil || provider.calls != 0 || repository.markedSuccess != nil {
		t.Fatal("native activation fell back to unsafe completion", err)
	}
}
