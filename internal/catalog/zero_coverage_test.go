package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type queuedPurchaseRepositoryStub struct {
	*catalogRepository
	purchase   model.Purchase
	err        error
	userID     string
	purchaseID string
	reason     string
	at         time.Time
}

func (r *queuedPurchaseRepositoryStub) CancelQueuedPurchase(_ context.Context, userID, purchaseID, reason string, at time.Time) (model.Purchase, error) {
	r.userID, r.purchaseID, r.reason, r.at = userID, purchaseID, reason, at
	return r.purchase, r.err
}

func TestCancelQueuedPurchase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := newCatalogServiceForTest(&catalogRepository{}, &catalogProxy{})
	for _, test := range []struct {
		name string
		user model.User
		id   string
	}{
		{name: "incomplete onboarding", user: model.User{ID: "user", OnboardingState: "agreement"}, id: "purchase"},
		{name: "blank purchase", user: model.User{ID: "user", OnboardingState: "complete"}, id: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.CancelQueuedPurchase(ctx, test.user, test.id); err == nil {
				t.Fatal("CancelQueuedPurchase() unexpectedly succeeded")
			}
		})
	}
	if _, err := service.CancelQueuedPurchase(ctx, model.User{ID: "user", OnboardingState: "complete"}, "purchase"); err == nil {
		t.Fatal("CancelQueuedPurchase() succeeded without a cancellation repository")
	}

	repository := &queuedPurchaseRepositoryStub{
		catalogRepository: &catalogRepository{},
		purchase:          model.Purchase{ID: "purchase", Status: "cancelled"},
		err:               errors.New("cancellation failure"),
	}
	service = newCatalogServiceForTest(repository, &catalogProxy{})
	user := model.User{ID: "user", OnboardingState: "complete"}
	if _, err := service.CancelQueuedPurchase(ctx, user, " purchase "); !errors.Is(err, repository.err) {
		t.Fatalf("CancelQueuedPurchase(error) = %v", err)
	}
	repository.err = nil
	purchase, err := service.CancelQueuedPurchase(ctx, user, " purchase ")
	if err != nil || purchase.ID != "purchase" || repository.userID != "user" || repository.purchaseID != "purchase" || repository.reason != "Queued entitlement cancelled by member" || !repository.at.Equal(service.now().UTC()) {
		t.Fatalf("CancelQueuedPurchase() = (%+v, %v), repository = %+v", purchase, err, repository)
	}
}

type renewalRepositoryStub struct {
	*catalogRepository
	quote      model.RenewalQuote
	quoteErr   error
	quoteInput struct {
		userID     string
		purchaseID string
		termCount  int
		at         time.Time
	}
	renewal      model.RenewalBatch
	renewalErr   error
	renewalInput database.RenewalInput
	renewalAt    time.Time
	excluded     []string
}

func (r *renewalRepositoryStub) RenewalQuote(_ context.Context, userID, purchaseID string, termCount int, at time.Time) (model.RenewalQuote, error) {
	r.quoteInput.userID, r.quoteInput.purchaseID, r.quoteInput.termCount, r.quoteInput.at = userID, purchaseID, termCount, at
	return r.quote, r.quoteErr
}

func (r *renewalRepositoryStub) Renew(_ context.Context, input database.RenewalInput, at time.Time) (model.RenewalBatch, error) {
	r.renewalInput, r.renewalAt = input, at
	return r.renewal, r.renewalErr
}

func (r *renewalRepositoryStub) RenewalQuoteExcludingAddons(_ context.Context, userID, purchaseID string, termCount int, excluded []string, at time.Time) (model.RenewalQuote, error) {
	r.quoteInput.userID, r.quoteInput.purchaseID, r.quoteInput.termCount, r.quoteInput.at = userID, purchaseID, termCount, at
	r.excluded = append([]string(nil), excluded...)
	blocked := make(map[string]struct{}, len(excluded))
	for _, id := range excluded {
		blocked[id] = struct{}{}
	}
	quote := r.quote
	quote.AddonSquadUUIDs = make([]string, 0, len(r.quote.AddonSquadUUIDs))
	for _, id := range r.quote.AddonSquadUUIDs {
		if _, skip := blocked[id]; !skip {
			quote.AddonSquadUUIDs = append(quote.AddonSquadUUIDs, id)
		}
	}
	return quote, r.quoteErr
}

func (r *renewalRepositoryStub) RenewExcludingAddons(_ context.Context, input database.RenewalInput, excluded []string, at time.Time) (model.RenewalBatch, error) {
	r.renewalInput, r.renewalAt = input, at
	r.excluded = append([]string(nil), excluded...)
	return r.renewal, r.renewalErr
}

func renewalTestRepository() *renewalRepositoryStub {
	return &renewalRepositoryStub{
		catalogRepository: &catalogRepository{combos: []model.Combo{{ID: "combo", IncludedSquads: []model.SquadProduct{{GroupID: "core-squad"}}}}},
		quote:             model.RenewalQuote{PurchaseID: "purchase", ComboID: "combo", TermCount: 2, TotalPrice: model.TXBMoney(200)},
	}
}

func renewalTestRemote() *catalogNodeRemote {
	return &catalogNodeRemote{
		catalogProxy: &catalogProxy{},
		squads:       []RemoteSquad{{UUID: "core-squad", Name: "Core"}},
		accessible:   map[string][]string{"core-squad": {"node-1"}},
		nodes:        []RemoteNode{{UUID: "node-1", Name: "Node", CountryCode: "US"}},
	}
}

func TestRenewalQuoteAndRenew(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := model.User{ID: "user", OnboardingState: "complete"}
	for _, test := range []struct {
		name string
		user model.User
		id   string
		term int
	}{
		{name: "incomplete onboarding", user: model.User{ID: "user"}, id: "purchase", term: 1},
		{name: "blank purchase", user: user, id: " ", term: 1},
		{name: "zero terms", user: user, id: "purchase", term: 0},
		{name: "too many terms", user: user, id: "purchase", term: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newCatalogServiceForTest(renewalTestRepository(), renewalTestRemote()).RenewalQuote(ctx, test.user, test.id, test.term); err == nil {
				t.Fatal("RenewalQuote() unexpectedly succeeded")
			}
		})
	}
	if _, err := newCatalogServiceForTest(&catalogRepository{}, renewalTestRemote()).RenewalQuote(ctx, user, "purchase", 1); err == nil {
		t.Fatal("RenewalQuote() succeeded without a renewal repository")
	}

	repository := renewalTestRepository()
	service := newCatalogServiceForTest(repository, renewalTestRemote())
	quote, err := service.RenewalQuote(ctx, user, " purchase ", 2)
	if err != nil || quote.ComboID != "combo" || len(quote.AccessibleNodes) != 1 || repository.quoteInput.userID != "user" || repository.quoteInput.purchaseID != " purchase " {
		t.Fatalf("RenewalQuote() = (%+v, %v), repository = %+v", quote, err, repository)
	}
	if !repository.quoteInput.at.Equal(service.now().UTC()) {
		t.Fatalf("RenewalQuote() time = %s, want %s", repository.quoteInput.at, service.now().UTC())
	}

	repository.quoteErr = errors.New("quote failure")
	if _, err := service.RenewalQuote(ctx, user, "purchase", 1); !errors.Is(err, repository.quoteErr) {
		t.Fatalf("RenewalQuote(repository failure) = %v", err)
	}
	noNodes := renewalTestRemote()
	noNodes.accessible = map[string][]string{}
	if _, err := newCatalogServiceForTest(renewalTestRepository(), noNodes).RenewalQuote(ctx, user, "purchase", 1); !errors.Is(err, ErrNoAccessibleNodes) {
		t.Fatalf("RenewalQuote(no nodes) = %v, want ErrNoAccessibleNodes", err)
	}

	repository.quoteErr = nil
	repository.renewal = model.RenewalBatch{ID: "batch-1", PurchaseID: "purchase", TermCount: 2}
	batch, err := service.Renew(ctx, user, "purchase", 2, "renewal-key")
	if err != nil || batch.ID != "batch-1" || repository.renewalInput.UserID != "user" || repository.renewalInput.PurchaseID != "purchase" || repository.renewalInput.TermCount != 2 || repository.renewalInput.IdempotencyKey != "renewal-key" || !repository.renewalAt.Equal(service.now().UTC()) {
		t.Fatalf("Renew() = (%+v, %v), repository = %+v", batch, err, repository)
	}
	repository.renewalErr = errors.New("renewal failure")
	if _, err := service.Renew(ctx, user, "purchase", 1, "renewal-key"); !errors.Is(err, repository.renewalErr) {
		t.Fatalf("Renew(repository failure) = %v", err)
	}

	for _, key := range []string{"", strings.Repeat("x", 129)} {
		if _, err := service.Renew(ctx, user, "purchase", 1, key); err == nil {
			t.Fatalf("Renew(%q) unexpectedly succeeded", key)
		}
	}
	quoteOnly := &renewalQuoteOnlyRepository{
		catalogRepository: renewalTestRepository().catalogRepository,
		quote:             renewalTestRepository().quote,
	}
	if _, err := newCatalogServiceForTest(quoteOnly, renewalTestRemote()).Renew(ctx, user, "purchase", 1, "key"); err == nil {
		t.Fatal("Renew() succeeded without the commit repository method")
	}
}

type renewalQuoteOnlyRepository struct {
	*catalogRepository
	quote model.RenewalQuote
}

func (r *renewalQuoteOnlyRepository) RenewalQuote(context.Context, string, string, int, time.Time) (model.RenewalQuote, error) {
	return r.quote, nil
}

type autoRenewalFailure struct {
	purchaseID string
	reason     string
	at         time.Time
}

type dueAutoRenewalRepository struct {
	*automaticRenewalCatalogRepository
	candidates []database.DueAutoRenewal
	dueErr     error
	planErr    error
	commitErr  error
	commitIDs  []string
	failed     []autoRenewalFailure
	markErr    error
}

func (r *dueAutoRenewalRepository) DueAutoRenewals(context.Context, time.Time) ([]database.DueAutoRenewal, error) {
	return r.candidates, r.dueErr
}

func (r *dueAutoRenewalRepository) AutoRenewalPlan(context.Context, string, string, time.Time) (database.AutoRenewalPlan, error) {
	return r.plan, r.planErr
}

func (r *dueAutoRenewalRepository) CommitAutoRenewal(_ context.Context, purchaseID string, _ time.Time) (model.Purchase, error) {
	r.commitIDs = append(r.commitIDs, purchaseID)
	return model.Purchase{ID: purchaseID}, r.commitErr
}

func (r *dueAutoRenewalRepository) CommitAutoRenewalExcludingAddons(_ context.Context, purchaseID string, _ []string, _ time.Time) (model.Purchase, error) {
	r.commitIDs = append(r.commitIDs, purchaseID)
	return model.Purchase{ID: purchaseID}, r.commitErr
}

func (r *dueAutoRenewalRepository) MarkAutoRenewalFailed(_ context.Context, purchaseID, reason string, at time.Time) error {
	r.failed = append(r.failed, autoRenewalFailure{purchaseID: purchaseID, reason: reason, at: at})
	return r.markErr
}

func dueAutoRenewalRepositoryForPlan(plan database.AutoRenewalPlan) *dueAutoRenewalRepository {
	return &dueAutoRenewalRepository{
		automaticRenewalCatalogRepository: &automaticRenewalCatalogRepository{
			catalogRepository: &catalogRepository{combos: []model.Combo{plan.Combo}, balance: model.TXBMoney(plan.NetMinor)},
			plan:              plan,
		},
		candidates: []database.DueAutoRenewal{{PurchaseID: plan.Purchase.ID, UserID: plan.Purchase.UserID}},
	}
}

func eligibleAutoRenewalPlan() database.AutoRenewalPlan {
	return database.AutoRenewalPlan{
		Purchase: model.Purchase{ID: "purchase", UserID: "user"},
		Combo:    model.Combo{ID: "combo", IncludedSquads: []model.SquadProduct{{GroupID: "core-squad"}}},
		NetMinor: 100,
	}
}

func TestProcessDueAutoRenewalsAndFailureReasons(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if err := newCatalogServiceForTest(&catalogRepository{}, renewalTestRemote()).ProcessDueAutoRenewals(ctx, now); err != nil {
		t.Fatalf("ProcessDueAutoRenewals(without repository) = %v", err)
	}

	repository := dueAutoRenewalRepositoryForPlan(eligibleAutoRenewalPlan())
	service := newCatalogServiceForTest(repository, renewalTestRemote())
	repository.dueErr = errors.New("due lookup failure")
	if err := service.ProcessDueAutoRenewals(ctx, now); !errors.Is(err, repository.dueErr) {
		t.Fatalf("ProcessDueAutoRenewals(due failure) = %v", err)
	}
	repository.dueErr = nil
	if err := service.ProcessDueAutoRenewals(ctx, now); err != nil || len(repository.commitIDs) != 1 || len(repository.failed) != 0 {
		t.Fatalf("ProcessDueAutoRenewals(success) = %v, commits=%v failures=%v", err, repository.commitIDs, repository.failed)
	}

	ineligible := eligibleAutoRenewalPlan()
	ineligible.IneligibleReason = database.AutoRenewalReasonQueuedPurchase
	repository = dueAutoRenewalRepositoryForPlan(ineligible)
	if err := newCatalogServiceForTest(repository, renewalTestRemote()).ProcessDueAutoRenewals(ctx, now); err != nil || len(repository.failed) != 1 || repository.failed[0].reason != database.AutoRenewalReasonQueuedPurchase {
		t.Fatalf("ProcessDueAutoRenewals(ineligible) = %v, failures=%v", err, repository.failed)
	}

	repository = dueAutoRenewalRepositoryForPlan(eligibleAutoRenewalPlan())
	repository.planErr = errors.New("plan failure")
	if err := newCatalogServiceForTest(repository, renewalTestRemote()).ProcessDueAutoRenewals(ctx, now); err != nil || len(repository.failed) != 1 || repository.failed[0].reason != database.AutoRenewalReasonUnavailable {
		t.Fatalf("ProcessDueAutoRenewals(plan failure) = %v, failures=%v", err, repository.failed)
	}

	repository = dueAutoRenewalRepositoryForPlan(eligibleAutoRenewalPlan())
	repository.commitErr = errors.New("commit failure")
	if err := newCatalogServiceForTest(repository, renewalTestRemote()).ProcessDueAutoRenewals(ctx, now); err != nil || len(repository.failed) != 1 || repository.failed[0].reason != database.AutoRenewalReasonUnavailable {
		t.Fatalf("ProcessDueAutoRenewals(commit failure) = %v, failures=%v", err, repository.failed)
	}

	repository = dueAutoRenewalRepositoryForPlan(eligibleAutoRenewalPlan())
	repository.markErr = errors.New("mark failure")
	repository.automaticRenewalCatalogRepository.plan.IneligibleReason = database.AutoRenewalReasonQueuedPurchase
	if err := newCatalogServiceForTest(repository, renewalTestRemote()).ProcessDueAutoRenewals(ctx, now); !errors.Is(err, repository.markErr) {
		t.Fatalf("ProcessDueAutoRenewals(mark failure) = %v", err)
	}

	repository = dueAutoRenewalRepositoryForPlan(eligibleAutoRenewalPlan())
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := newCatalogServiceForTest(repository, renewalTestRemote()).ProcessDueAutoRenewals(cancelled, now); !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessDueAutoRenewals(cancelled) = %v, want context.Canceled", err)
	}

	ineligibleReason := database.AutoRenewalReasonQueuedPurchase
	if got := autoRenewalFailureReason(model.AutoRenewal{IneligibleReason: &ineligibleReason}, nil); got != ineligibleReason {
		t.Fatalf("autoRenewalFailureReason(ineligible) = %q", got)
	}
	if got := autoRenewalFailureReason(model.AutoRenewal{}, database.ErrInsufficientBalance); got != database.AutoRenewalReasonInsufficientBalance {
		t.Fatalf("autoRenewalFailureReason(balance) = %q", got)
	}
	if got := autoRenewalFailureReason(model.AutoRenewal{}, errors.New("unavailable")); got != database.AutoRenewalReasonUnavailable {
		t.Fatalf("autoRenewalFailureReason(generic) = %q", got)
	}
}

func TestProcessDueAutoRenewalsDoesNotBalanceGateBeforeSettlement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	plan := eligibleAutoRenewalPlan()
	plan.GrossMinor, plan.NetMinor = 100, 100
	repository := dueAutoRenewalRepositoryForPlan(plan)
	repository.balance = model.TXBMoney(50)
	if err := newCatalogServiceForTest(repository, renewalTestRemote()).ProcessDueAutoRenewals(ctx, now); err != nil {
		t.Fatalf("ProcessDueAutoRenewals() = %v", err)
	}
	if len(repository.commitIDs) != 1 || len(repository.failed) != 0 {
		t.Fatalf("renewal commits=%v failures=%v, want settlement-time balance handling", repository.commitIDs, repository.failed)
	}
}
