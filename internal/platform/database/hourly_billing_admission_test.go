package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHourlyActivationRequiresExistingSquadCodeAtBothPhases(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	product := SquadProductInput{ID: input.GroupID, GroupID: input.GroupID, Name: "Gated hourly", UpstreamPresent: true,
		Visible: true, ActivationRequired: true, ActivationCode: "first-hourly-code"}
	if _, err := s.SaveSquadProduct(ctx, product); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareHourlyActivation(ctx, input, policy, time.Now()); !errors.Is(err, ErrActivationCodeRequired) {
		t.Fatalf("hourly entry bypassed required squad code: %v", err)
	}
	input.ActivationCode = "wrong-code"
	if _, err := s.PrepareHourlyActivation(ctx, input, policy, time.Now()); !errors.Is(err, ErrActivationCodeInvalid) {
		t.Fatalf("hourly entry accepted invalid code: %v", err)
	}
	input.ActivationCode = "first-hourly-code"
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	product.ActivationCode = "rotated-hourly-code"
	if _, err := s.SaveSquadProduct(ctx, product); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Now()); !errors.Is(err, ErrActivationCodeInvalid) {
		t.Fatalf("commit bypassed code rotation: %v", err)
	}
	if hourlyBillingBalance(t, s, input.OwnerID) != 100000 {
		t.Fatal("failed squad-code admission charged wallet")
	}
	prepared.ActivationCode = "rotated-hourly-code"
	prepared.Delivery = hourlyBillingDelivery(t, s, prepared.EnrollmentID)[0]
	prepared.ExpectedCompileRevision = prepared.Delivery.CompileRevision
	if _, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Now()); err != nil {
		t.Fatalf("fresh existing activation code could not complete prepared intent: %v", err)
	}
}

func TestHourlyCancellationCannotUndoAConcurrentPaidCommit(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	ctx := context.Background()
	start, commitDone, cancelDone := make(chan struct{}), make(chan error, 1), make(chan error, 1)
	go func() {
		<-start
		_, err := s.CommitHourlyActivation(ctx, prepared, policy, time.Now())
		commitDone <- err
	}()
	go func() {
		<-start
		_, err := s.CancelHourlyPreparation(ctx, input.OwnerID, input.AccountGeneration, prepared.EnrollmentID, time.Now())
		cancelDone <- err
	}()
	close(start)
	commitErr, cancelErr := <-commitDone, <-cancelDone
	e, err := s.HourlyEnrollment(ctx, prepared.EnrollmentID)
	if err != nil {
		t.Fatal(err)
	}
	if commitErr == nil {
		if !errors.Is(cancelErr, ErrConflict) || e.Status != "active" || hourlyBillingBalance(t, s, input.OwnerID) >= 100000 {
			t.Fatalf("cancellation undid an accepted payment: %+v %v", e, cancelErr)
		}
	} else if cancelErr == nil {
		if !errors.Is(commitErr, ErrConflict) || e.Status != "disabled" || e.PaidThrough != nil || e.CooldownUntil != nil || hourlyBillingBalance(t, s, input.OwnerID) != 100000 {
			t.Fatalf("cancelled unpaid intent charged/granted access: %+v %v", e, commitErr)
		}
	} else {
		t.Fatalf("neither serialized transition succeeded: commit=%v cancel=%v", commitErr, cancelErr)
	}
}
