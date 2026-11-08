package hourlybilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestExtraFundingReconcilesOriginalReceiptBeforeNewPriceOrDemand(t *testing.T) {
	s, user, policy, _ := commerceFixture(t)
	op, job := queuedActivation(t, s, user, policy)
	if err := s.HandleProviderOperation(context.Background(), op, job); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := s.store.EnsureHourlyCreditDay(ctx, user.ID, user.AccountGeneration, now.Format(time.DateOnly), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.SetHourlyExtraPreference(ctx, database.HourlyExtraPreferenceInput{OwnerID: user.ID, AccountGeneration: user.AccountGeneration, Enabled: true, AcceptedPriceTXBMinor: 25}, policy.snapshot, now); err != nil {
		t.Fatal(err)
	}
	request := xnode.HourlyExtraRequest{OwnerID: user.ID, AccountGeneration: user.AccountGeneration, DayUTC: now.Format(time.DateOnly), NodeID: "node", PendingBytes: 100, PendingObservedAt: now}
	scope := fundScope{user.ID, user.AccountGeneration, request.DayUTC}
	intent, err := s.fundingIntent(ctx, scope, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.store.FundHourlyExtraGB(ctx, intent.input, intent.snapshot, now); err != nil {
		t.Fatal(err)
	}
	paid := balanceFixture(t, s, user)
	// Model a lost COMMIT reply. RAM retains the exact receipt/terms, and the
	// next real demand may not replace either before resolving that outcome.
	intent.running = false
	s.funds[scope] = intent
	policy.err = errors.New("new policy unavailable")
	request.PendingBytes, request.PendingObservedAt = 200, time.Now().UTC()
	if err = s.BuyHourlyExtra(ctx, request); err != nil {
		t.Fatal(err)
	}
	day, err := s.store.HourlyCreditDay(ctx, user.ID, user.AccountGeneration, request.DayUTC)
	if err != nil {
		t.Fatal(err)
	}
	if balanceFixture(t, s, user) != paid || day.ExtraFundedBytes != 1_000_000_000 || len(s.funds) != 0 {
		t.Fatal("receipt replay duplicated debit/credit or lost uncertainty")
	}
}

func TestFundingIntentRetainsOriginalFreshnessAndSerializesOneOwner(t *testing.T) {
	policy := &fixturePolicy{snapshot: model.HourlyPolicySnapshot{Version: "version"}}
	s := New(nil, policy, nil, nil)
	now := time.Now().UTC()
	request := xnode.HourlyExtraRequest{OwnerID: "opaque-owner", AccountGeneration: 2, DayUTC: now.Format(time.DateOnly), PendingBytes: 1, PendingObservedAt: now}
	scope := fundScope{request.OwnerID, request.AccountGeneration, request.DayUTC}
	first, err := s.fundingIntent(context.Background(), scope, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.fundingIntent(context.Background(), scope, request); err == nil {
		t.Fatal("concurrent funding intent was admitted")
	}
	first.running = false
	s.funds[scope] = first
	request.PendingObservedAt = now.Add(time.Second)
	policy.err = errors.New("policy changed")
	replay, err := s.fundingIntent(context.Background(), scope, request)
	if err != nil || replay.id != first.id || replay.input.PendingObservedAt != now || policy.reads != 1 {
		t.Fatal("uncertain receipt, original timestamp or reviewed policy was rewritten")
	}
	request.PendingObservedAt = now.Add(-5 * time.Second)
	if err = s.BuyHourlyExtra(context.Background(), request); !errors.Is(err, database.ErrHourlyDemandStale) {
		t.Fatalf("stale request = %v", err)
	}
}
