package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHourlyPreparationIsUnpaidAndConcurrentCommitReplays(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	if got := hourlyBillingBalance(t, s, input.OwnerID); got != 100000 {
		t.Fatalf("preparation charged balance: %d", got)
	}
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan HourlyActivationResult, 2)
	errorsCh := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			result, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Time{})
			results <- result
			errorsCh <- err
		}()
	}
	close(start)
	a, b := <-results, <-results
	for range 2 {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	if a.Settlement == nil || b.Settlement == nil || a.Settlement.ID != b.Settlement.ID || a.Enrollment.ID != b.Enrollment.ID {
		t.Fatalf("concurrent commits did not replay one receipt: %+v %+v", a, b)
	}
	want, err := hourlyCharge(100, a.Settlement.ChargedFrom, a.Settlement.PaidUntil)
	if err != nil || a.Settlement.ChargedTXBMinor != want || hourlyBillingBalance(t, s, input.OwnerID) != 100000-want {
		t.Fatalf("prorated debit differs: %+v, %v", a.Settlement, err)
	}
	if !a.Enrollment.ActivatedAt.Equal(a.Settlement.ChargedFrom) || !a.Enrollment.PaidThrough.Equal(a.Settlement.PaidUntil) {
		t.Fatal("native paid boundary differs from immutable receipt")
	}
	if _, err := s.CompareAndSwapHourlyPolicy(ctx, hourlyBillingAdmin(t, s), policy.Version, "changed-policy-cipher"); err != nil {
		t.Fatal(err)
	}
	replay, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Now())
	if err != nil || replay.Settlement.ID != a.Settlement.ID || hourlyBillingBalance(t, s, input.OwnerID) != 100000-want {
		t.Fatalf("changed policy broke accepted replay: %+v %v", replay, err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE kind='hourly_charge'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("charge receipts = %d, %v", count, err)
	}
}

func TestHourlyActivationRechecksThresholdAndNativeTuple(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	wrong := prepared
	wrong.PeriodID = "foreign-period"
	if _, err := s.CommitHourlyActivation(context.Background(), wrong, policy, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("foreign prepared tuple = %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=16799 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitHourlyActivation(context.Background(), prepared, policy, time.Now()); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("stale eligibility allowed activation: %v", err)
	}
	if hourlyBillingBalance(t, s, input.OwnerID) != 16799 {
		t.Fatal("failed activation debited balance")
	}
	e, err := s.HourlyEnrollment(context.Background(), prepared.EnrollmentID)
	if err != nil || e.ActivatedAt != nil || e.PaidThrough != nil || e.Status != "preparing" {
		t.Fatalf("failed activation mutated access: %+v %v", e, err)
	}
}
