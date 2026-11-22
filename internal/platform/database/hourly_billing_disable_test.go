package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyVoluntaryDisableRetainsChargeAndAllowsImmediateRepurchase(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	active := activateHourlyBillingFixture(t, s, input, policy)
	ctx := context.Background()
	balance := hourlyBillingBalance(t, s, input.OwnerID)
	disable := HourlyDisableInput{OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration, EnrollmentID: active.Enrollment.ID, Voluntary: true}
	e, err := s.DisableHourlyEnrollment(ctx, disable, policy, time.Time{})
	if err != nil || e.Status != model.HourlyDisabled || e.DisabledAt == nil || e.CooldownUntil != nil || !e.PaidThrough.Equal(*active.Enrollment.PaidThrough) {
		t.Fatalf("disable consequences differ: %+v %v", e, err)
	}
	replay, err := s.DisableHourlyEnrollment(ctx, disable, policy, time.Now().Add(time.Hour))
	if err != nil || replay.CooldownUntil != nil || !replay.DisabledAt.Equal(*e.DisabledAt) || hourlyBillingBalance(t, s, input.OwnerID) != balance {
		t.Fatalf("disable replay changed cooldown/charge: %+v %v", replay, err)
	}
	// An unexpired wait written by an older release must not block a new purchase.
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET cooldown_until=clock_timestamp()+interval '1 day' WHERE id=$1`, e.ID); err != nil {
		t.Fatal(err)
	}
	input.ActivationKey = "reenable"
	next := activateHourlyBillingFixture(t, s, input, policy)
	if next.Enrollment.ID == e.ID || hourlyBillingBalance(t, s, input.OwnerID) != balance-168*100 {
		t.Fatal("immediate reactivation must buy its own minimum interval")
	}
}

func TestHourlyFundingSuspensionAndPreparationCancellationHaveNoCooldown(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	active := activateHourlyBillingFixture(t, s, input, policy)
	ctx := context.Background()
	e, err := s.DisableHourlyEnrollment(ctx, HourlyDisableInput{OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration,
		EnrollmentID: active.Enrollment.ID}, policy, time.Time{})
	if err != nil || e.Status != model.HourlyFundingSuspended || e.CooldownUntil != nil {
		t.Fatalf("automatic funding suspension imposed cooldown: %+v %v", e, err)
	}
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=100 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	input.ActivationKey = "explicit-reenable"
	if _, err := s.PrepareHourlyActivation(ctx, input, policy, time.Now()); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("reenable bypassed 168-hour eligibility: %v", err)
	}
	if _, err := s.AdjustBalance(ctx, input.OwnerID, 100000, "reenable-funds", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	prepared, err := s.PrepareHourlyActivation(ctx, input, policy, time.Now())
	if err != nil || prepared.ID == active.Enrollment.ID {
		t.Fatalf("reenable reused old activation: %+v %v", prepared, err)
	}
	cancelled, err := s.DisableHourlyEnrollment(ctx, HourlyDisableInput{OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration,
		EnrollmentID: prepared.ID, Voluntary: true}, policy, time.Now())
	if err != nil || cancelled.CooldownUntil != nil || cancelled.ActivatedAt != nil {
		t.Fatalf("unpaid cancellation created cooldown/benefit: %+v %v", cancelled, err)
	}
}
