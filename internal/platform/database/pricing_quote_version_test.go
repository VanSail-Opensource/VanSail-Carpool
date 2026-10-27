package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReviewedPriceChangeDoesNotDebitAndReplaySurvivesExpiry(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 49551)
	now := time.Now().UTC().Truncate(5 * time.Minute).Add(time.Minute)
	combo := saveTestCombo(t, s, "reviewed", 1000, 30)
	if _, err := s.AdjustBalance(ctx, user.ID, 5000, "review-funds", "seed", now); err != nil {
		t.Fatal(err)
	}
	input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "reviewed"}
	q, err := s.QuotePurchase(ctx, input, now)
	if err != nil || q.QuoteVersion == "" {
		t.Fatal(q, err)
	}
	input.QuoteVersion = q.QuoteVersion
	if _, err = s.db.ExecContext(ctx, `UPDATE combos SET price_txb_minor=1100 WHERE id=?`, combo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreatePurchase(ctx, input, now); !errors.Is(err, ErrPricingQuoteChanged) {
		t.Fatal(err)
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.MinorInt64() != 5000 {
		t.Fatal(balance, err)
	}
	q, err = s.QuotePurchase(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	input.QuoteVersion = q.QuoteVersion
	purchased, err := s.CreatePurchase(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.CreatePurchase(ctx, input, now.Add(24*time.Hour))
	if err != nil || replay.ID != purchased.ID {
		t.Fatal(replay, err)
	}
}
func TestDynamicQuoteRequiresVersionButFixedLegacyRequestDoesNot(t *testing.T) {
	for _, tc := range []struct {
		provided, current  string
		dynamic, wantError bool
	}{{"", "a", true, true}, {"old", "new", true, true}, {"a", "a", true, false}, {"", "a", false, false}, {"old", "new", false, true}} {
		if got := validateReviewedQuote(tc.provided, tc.current, tc.dynamic); errors.Is(got, ErrPricingQuoteChanged) != tc.wantError {
			t.Fatal(tc, got)
		}
	}
}
