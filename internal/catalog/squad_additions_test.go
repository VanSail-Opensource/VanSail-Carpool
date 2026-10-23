package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestSquadAdditionQuoteAndCommit(t *testing.T) {
	t.Parallel()

	repository := newPurchaseAddonCatalogRepository()
	service := newCatalogServiceForTest(repository, &catalogProxy{})
	user := model.User{ID: "member", OnboardingState: "complete"}
	quote, err := service.QuoteAddons(context.Background(), user, "purchase", []string{" squad-1 "})
	if err != nil || quote.PurchaseID != "purchase" || repository.quoteInput.UserID != user.ID || repository.quoteInput.PurchaseID != "purchase" || repository.quoteInput.AddonSquadIDs[0] != " squad-1 " || !repository.quoteAt.Equal(service.now().UTC()) {
		t.Fatalf("QuoteAddons() = (%+v, %v), repository = %+v", quote, err, repository)
	}

	purchase, err := service.AddAddons(context.Background(), user, "purchase", []string{"squad-1"}, map[string]string{"squad-1": "code"}, "addition-key")
	if err != nil || purchase.ID != "purchase" || repository.addInput.UserID != user.ID || repository.addInput.PurchaseID != "purchase" || repository.addInput.IdempotencyKey != "addition-key" || repository.addInput.SquadActivationCodes["squad-1"] != "code" || !repository.addAt.Equal(service.now().UTC()) {
		t.Fatalf("AddAddons() = (%+v, %v), repository = %+v", purchase, err, repository)
	}

	repository.quoteErr = errors.New("quote failure")
	if _, err := service.QuoteAddons(context.Background(), user, "purchase", []string{"squad-1"}); !errors.Is(err, repository.quoteErr) {
		t.Fatalf("QuoteAddons(repository failure) = %v", err)
	}
	repository.addErr = errors.New("commit failure")
	if _, err := service.AddAddons(context.Background(), user, "purchase", []string{"squad-1"}, nil, "addition-key"); !errors.Is(err, repository.addErr) {
		t.Fatalf("AddAddons(repository failure) = %v", err)
	}
}

func TestSquadAdditionValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	complete := model.User{ID: "member", OnboardingState: "complete"}
	service := newCatalogServiceForTest(newPurchaseAddonCatalogRepository(), &catalogProxy{})
	for _, user := range []model.User{{ID: "member"}, complete} {
		addonIDs := []string{"squad-1"}
		if user.OnboardingState == "complete" {
			addonIDs = nil
		}
		if _, err := service.QuoteAddons(ctx, user, "purchase", addonIDs); err == nil {
			t.Fatalf("QuoteAddons(%+v, %v) unexpectedly succeeded", user, addonIDs)
		}
	}
	if _, err := service.AddAddons(ctx, complete, "purchase", make([]string, 101), nil, "key"); err == nil {
		t.Fatal("AddAddons() accepted too many squads")
	}

	listFailure := newPurchaseAddonCatalogRepository()
	listFailure.addonsErr = errors.New("catalog failure")
	if _, err := newCatalogServiceForTest(listFailure, &catalogProxy{}).QuoteAddons(ctx, complete, "purchase", []string{"squad-1"}); !errors.Is(err, listFailure.addonsErr) {
		t.Fatalf("QuoteAddons(catalog failure) = %v", err)
	}
	for _, product := range []model.SquadProduct{{GroupID: "squad-1", Visible: false, UpstreamPresent: true}, {GroupID: "squad-1", Visible: true}} {
		repository := newPurchaseAddonCatalogRepository()
		repository.addons = []model.SquadProduct{product}
		if _, err := newCatalogServiceForTest(repository, &catalogProxy{}).QuoteAddons(ctx, complete, "purchase", []string{"squad-1"}); !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("QuoteAddons(invisible product) = %v", err)
		}
	}

	unsupported := &catalogRepository{addons: []model.SquadProduct{{GroupID: "squad-1", Visible: true, UpstreamPresent: true}}}
	if _, err := newCatalogServiceForTest(unsupported, &catalogProxy{}).QuoteAddons(ctx, complete, "purchase", []string{"squad-1"}); err == nil {
		t.Fatal("QuoteAddons() succeeded without an add-on repository")
	}
}

type purchaseAddonCatalogRepository struct {
	*catalogRepository
	quote      model.PurchaseAddonQuote
	quoteErr   error
	quoteInput database.PurchaseAddonInput
	quoteAt    time.Time
	add        model.Purchase
	addErr     error
	addInput   database.PurchaseAddonInput
	addAt      time.Time
}

func newPurchaseAddonCatalogRepository() *purchaseAddonCatalogRepository {
	return &purchaseAddonCatalogRepository{
		catalogRepository: &catalogRepository{addons: []model.SquadProduct{{GroupID: "squad-1", Visible: true, UpstreamPresent: true}}},
		quote:             model.PurchaseAddonQuote{PurchaseID: "purchase"},
		add:               model.Purchase{ID: "purchase"},
	}
}

func (r *purchaseAddonCatalogRepository) QuotePurchaseAddons(_ context.Context, input database.PurchaseAddonInput, at time.Time) (model.PurchaseAddonQuote, error) {
	r.quoteInput, r.quoteAt = input, at
	return r.quote, r.quoteErr
}

func (r *purchaseAddonCatalogRepository) AddPurchaseAddons(_ context.Context, input database.PurchaseAddonInput, at time.Time) (model.Purchase, error) {
	r.addInput, r.addAt = input, at
	return r.add, r.addErr
}
