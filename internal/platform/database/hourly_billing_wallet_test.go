package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHourlyActivationAndConcurrentWalletSpendingCannotOverdraw(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	ctx := context.Background()
	start, committed, spent := make(chan struct{}), make(chan error, 1), make(chan error, 1)
	go func() {
		<-start
		_, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Now())
		committed <- err
	}()
	go func() {
		<-start
		_, err := s.DeductBalance(ctx, input.OwnerID, 100000, "concurrent-hourly-spend", "fixture", time.Now())
		spent <- err
	}()
	close(start)
	commitErr, spendErr := <-committed, <-spent
	if commitErr == nil {
		if !errors.Is(spendErr, ErrInsufficientBalance) || hourlyBillingBalance(t, s, input.OwnerID) < 99900 {
			t.Fatalf("spending ignored already-paid hour: commit=%v spend=%v", commitErr, spendErr)
		}
	} else if spendErr == nil {
		if !errors.Is(commitErr, ErrInsufficientBalance) || hourlyBillingBalance(t, s, input.OwnerID) != 0 {
			t.Fatalf("activation ignored concurrent wallet spend: commit=%v spend=%v", commitErr, spendErr)
		}
	} else {
		t.Fatalf("neither valid serialized spend succeeded: commit=%v spend=%v", commitErr, spendErr)
	}
}

func TestHourlyDistinctFundingWorkersBuyOnlyOneUnconsumedBlock(t *testing.T) {
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
	first := freshHourlyFunding(t, s, input, policy, credit.DayUTC, "parallel-extra-a")
	second := first
	second.ReceiptID = "parallel-extra-b"
	start, done := make(chan struct{}), make(chan error, 2)
	for _, request := range []HourlyExtraFundingInput{first, second} {
		go func() {
			<-start
			_, err := s.FundHourlyExtraGB(ctx, request, policy, time.Now())
			done <- err
		}()
	}
	close(start)
	a, b := <-done, <-done
	if !(a == nil && errors.Is(b, ErrHourlyNotExhausted) || b == nil && errors.Is(a, ErrHourlyNotExhausted)) {
		t.Fatalf("distinct workers bought ahead of confirmed usage: %v %v", a, b)
	}
	if day := readHourlyDayFixture(t, s, credit); day.ExtraFundedBytes != hourlyExtraBlockBytes {
		t.Fatalf("concurrent funding minted extra blocks: %+v", day)
	}
}
