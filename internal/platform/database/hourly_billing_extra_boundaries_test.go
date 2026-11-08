package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHourlyExtraFundingRejectsStaleDemandAndInsufficientFundsWithoutMutation(t *testing.T) {
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
	funding := freshHourlyFunding(t, s, input, policy, credit.DayUTC, "bounded-extra")
	funding.PendingObservedAt = funding.PendingObservedAt.Add(-5 * time.Second)
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrHourlyDemandStale) {
		t.Fatalf("stale pending payload purchased traffic: %v", err)
	}
	funding = freshHourlyFunding(t, s, input, policy, credit.DayUTC, "bounded-extra")
	funding.PendingBytes = 0
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrHourlyDemandStale) {
		t.Fatalf("zero pending payload purchased traffic: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=24 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	funding = freshHourlyFunding(t, s, input, policy, credit.DayUTC, "bounded-extra")
	if _, err := s.FundHourlyExtraGB(ctx, funding, policy, time.Time{}); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("extra purchase overdrew wallet: %v", err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM hourly_extra_purchases`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected funding retained a receipt: %d %v", count, err)
	}
	day := readHourlyDayFixture(t, s, credit)
	if day.ExtraFundedBytes != 0 || hourlyBillingBalance(t, s, input.OwnerID) != 24 {
		t.Fatalf("rejected funding changed exact accounting: %+v", day)
	}
}
