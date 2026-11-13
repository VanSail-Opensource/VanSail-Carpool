package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHourlyExtrasRequireExactExhaustionAndReplayOneDecimalGB(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	activateHourlyBillingFixture(t, s, input, policy)
	credit := prepareHourlyBillingCredit(t, s, input, policy)
	ctx := context.Background()
	funding := freshHourlyFunding(t, s, input, policy, credit.DayUTC, "extra-first")
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrHourlyExtraDisabled) {
		t.Fatalf("default opt-out did not block extra purchase: %v", err)
	}
	optIntoHourlyExtras(t, s, input, policy)
	reserved := reserveHourlyFixture(t, s, credit)
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrHourlyNotExhausted) {
		t.Fatalf("reserved but unconsumed free allowance triggered purchase: %v", err)
	}
	if _, err := s.ApplyHourlyCreditProof(ctx, credit.NodeID, credit.AgentID, hourlyProofFixture(t, reserved, 500, 500, 0, 0, false)); err != nil {
		t.Fatal(err)
	}
	funding = freshHourlyFunding(t, s, input, policy, credit.DayUTC, "extra-first")
	before := hourlyBillingBalance(t, s, input.OwnerID)
	start, receipts, failures := make(chan struct{}), make(chan string, 2), make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			r, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{})
			receipts <- r.LedgerID
			failures <- err
		}()
	}
	close(start)
	a, b := <-receipts, <-receipts
	for range 2 {
		if err := <-failures; err != nil {
			t.Fatal(err)
		}
	}
	day := readHourlyDayFixture(t, s, credit)
	if a == "" || a != b || day.ExtraFundedBytes != 1_000_000_000 || hourlyBillingBalance(t, s, input.OwnerID) != before-25 {
		t.Fatalf("duplicate funding/incorrect GB: %q %q %+v", a, b, day)
	}
	if _, err := s.SetHourlyExtraPreference(ctx, HourlyExtraPreferenceInput{OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration}, policy, time.Now()); err != nil {
		t.Fatal(err)
	}
	funding.PendingBytes, funding.PendingObservedAt = 0, time.Time{}
	if replay, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); err != nil || replay.LedgerID != a {
		t.Fatalf("accepted replay depended on new consent/demand: %+v %v", replay, err)
	}
	credit.ReservationID, credit.Kind, credit.ByteCeiling = "billing-extra-credit", "extra", 1_000_000_000
	unknown := reserveHourlyFixture(t, s, credit)
	if _, err := s.ApplyHourlyCreditProof(ctx, credit.NodeID, credit.AgentID, hourlyProofFixture(t, unknown, 0, 0, 0, 1_000_000_000, true)); err != nil {
		t.Fatal(err)
	}
	optIntoHourlyExtras(t, s, input, policy)
	funding = freshHourlyFunding(t, s, input, policy, credit.DayUTC, "extra-after-unknown")
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrHourlyNotExhausted) {
		t.Fatalf("unknown funded debt counted as usage exhaustion: %v", err)
	}
}

func TestHourlyExtraPriceIncreaseRequiresNewConsentAndDecreaseUsesActualPrice(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	activateHourlyBillingFixture(t, s, input, policy)
	credit := prepareHourlyBillingCredit(t, s, input, policy)
	reserved := reserveHourlyFixture(t, s, credit)
	ctx := context.Background()
	if _, err := s.ApplyHourlyCreditProof(ctx, credit.NodeID, credit.AgentID, hourlyProofFixture(t, reserved, 1000, 0, 0, 0, false)); err != nil {
		t.Fatal(err)
	}
	optIntoHourlyExtras(t, s, input, policy)
	record, err := s.CompareAndSwapHourlyPolicy(ctx, hourlyBillingAdmin(t, s), policy.Version, "higher-extra-price")
	if err != nil {
		t.Fatal(err)
	}
	policy.Version, policy.Policy.ExtraGBPriceTXBMinor = record.Version, 50
	funding := freshHourlyFunding(t, s, input, policy, credit.DayUTC, "price-cap")
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrHourlyExtraDisabled) {
		t.Fatalf("higher automatic price bypassed consent: %v", err)
	}
	record, err = s.CompareAndSwapHourlyPolicy(ctx, hourlyBillingAdmin(t, s), policy.Version, "lower-extra-price")
	if err != nil {
		t.Fatal(err)
	}
	policy.Version, policy.Policy.ExtraGBPriceTXBMinor = record.Version, 10
	funding = freshHourlyFunding(t, s, input, policy, credit.DayUTC, "price-cap")
	purchase, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{})
	if err != nil || purchase.UnitPriceTXBMinor != 10 || purchase.ByteCount != 1_000_000_000 {
		t.Fatalf("lower price did not charge actual current price: %+v %v", purchase, err)
	}
}
