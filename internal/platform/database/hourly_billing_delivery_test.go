package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The bounded prefetch path is disabled here (sequential, zero lead window) so
// the retry observes only the late current-remainder rule; otherwise a sibling
// boundary that legitimately falls inside the prefetch window would also be
// skipped for its missing proof and the stale-squad assertion would be flaky.
func TestHourlyStaleDeliverySkipsOnlyThatSquadWithoutBackbilling(t *testing.T) {
	withHourlyPrefetchLead(t, 0)
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	now := hourlyBillingNow(t, s)
	a := makeDueHourlyBillingFixture(t, s, input, policy, now.Add(-5*time.Hour))
	second := input
	second.GroupID, second.ActivationKey = "billing-hourly-b", "available-sibling"
	addHourlyBillingOffer(t, s, hourlyBillingAdmin(t, s), second.GroupID, 100)
	b := makeDueHourlyBillingFixture(t, s, second, policy, now.Add(-4*time.Hour))
	guards := hourlyBillingDelivery(t, s, a.ID, b.ID)
	guards[0].ObservedAt = guards[0].ObservedAt.Add(-5 * time.Second)
	guards[0].ReadyUntil = guards[0].ObservedAt.Add(4 * time.Second)
	before := hourlyBillingBalance(t, s, input.OwnerID)
	result, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy, guards, guards[0].ObservedAt)
	if err != nil || len(result.Settlements) != 1 || result.Settlements[0].EnrollmentID != b.ID ||
		len(result.SkippedEnrollmentIDs) != 1 || result.SkippedEnrollmentIDs[0] != a.ID {
		t.Fatalf("unavailable squad starved available sibling or charged stale proof: %+v %v", result, err)
	}
	stillWaiting, err := s.HourlyEnrollment(ctx, a.ID)
	if err != nil || !stillWaiting.PaidThrough.Equal(*a.PaidThrough) || hourlyBillingBalance(t, s, input.OwnerID) != before-result.Settlements[0].ChargedTXBMinor {
		t.Fatalf("skipped squad consumed wallet/access: %+v %v", stillWaiting, err)
	}
	// A future successful retry buys only its then-current remainder; the stale
	// scheduler hint cannot move the immutable charged_from backward.
	result, err = s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy, hourlyBillingDelivery(t, s, a.ID), now.Add(-2*time.Hour))
	if err != nil || len(result.Settlements) != 1 || result.Settlements[0].EnrollmentID != a.ID || result.Settlements[0].ChargedFrom.Before(now) {
		t.Fatalf("available retry billed an unavailable interval: %+v %v", result, err)
	}
}

func TestHourlyInitialCommitRequiresFreshExactNativeReadiness(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	ctx := context.Background()
	stale := prepared
	stale.Delivery.ObservedAt = stale.Delivery.ObservedAt.Add(-5 * time.Second)
	stale.Delivery.ReadyUntil = stale.Delivery.ObservedAt.Add(4 * time.Second)
	if _, err := s.CommitHourlyActivation(ctx, stale, policy, stale.Delivery.ObservedAt); !errors.Is(err, ErrHourlyDeliveryPending) {
		t.Fatalf("stale initial preflight charged account: %v", err)
	}
	wrongRevision := prepared
	wrongRevision.ExpectedCompileRevision++
	if _, err := s.CommitHourlyActivation(ctx, wrongRevision, policy, time.Now()); !errors.Is(err, ErrHourlyDeliveryPending) {
		t.Fatalf("stale configuration preflight charged account: %v", err)
	}
	if hourlyBillingBalance(t, s, input.OwnerID) != 100000 {
		t.Fatal("unavailable initial service consumed balance")
	}
	prepared.Delivery = hourlyBillingDelivery(t, s, prepared.EnrollmentID)[0]
	prepared.ExpectedCompileRevision = prepared.Delivery.CompileRevision
	if _, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Now()); err != nil {
		t.Fatalf("fresh preflight could not finish same unpaid intent: %v", err)
	}
}
