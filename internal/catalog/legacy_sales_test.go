package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type closedSalesRepository struct {
	catalogRepository
	replay model.Purchase
	found  bool
	err    error
	input  database.PurchaseInput
}

func (r *closedSalesRepository) PurchaseReplay(_ context.Context, input database.PurchaseInput) (model.Purchase, bool, error) {
	r.input = input
	return r.replay, r.found, r.err
}

func TestClosedMemberSalesOnlyReturnExactReplay(t *testing.T) {
	r := &closedSalesRepository{}
	s := NewService(r, nil, 0)
	u := model.User{ID: "owner", OnboardingState: "complete"}
	input := database.PurchaseInput{UserID: "forged", ComboID: "old-combo", IdempotencyKey: "historical", CouponGrantID: "coupon", QuoteVersion: "terms"}
	if _, err := s.ReplayLegacyPurchase(context.Background(), u, input); !errors.Is(err, ErrLegacySalesClosed) {
		t.Fatal("new fixed-term sale was admitted", err)
	}
	r.found, r.replay = true, model.Purchase{ID: "committed"}
	got, err := s.ReplayLegacyPurchase(context.Background(), u, input)
	if err != nil || got.ID != "committed" || r.input.UserID != u.ID || r.input.CouponGrantID != "coupon" || r.input.QuoteVersion != "terms" {
		t.Fatal("historical replay lost its owner or original terms", err)
	}
	r.found, r.err = false, database.ErrConflict
	if _, err := s.ReplayLegacyPurchase(context.Background(), u, input); !errors.Is(err, database.ErrConflict) {
		t.Fatal("a changed replay request lost its conflict", err)
	}
	if !r.purchaseAt.IsZero() {
		t.Fatal("member replay entered the new-purchase transaction")
	}
}
