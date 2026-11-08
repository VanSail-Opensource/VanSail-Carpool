package hourlybilling

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	queue "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestPaidActivationReplayIgnoresNewPolicyAndNeverChargesTwice(t *testing.T) {
	s, user, policy, native := commerceFixture(t)
	op, job := queuedActivation(t, s, user, policy)
	native.syncError = errNativeWait
	var waiting *queue.WaitError
	if err := s.HandleProviderOperation(context.Background(), op, job); !errors.As(err, &waiting) {
		t.Fatalf("first apply = %v", err)
	}
	paid := balanceFixture(t, s, user)
	if paid >= 100000 {
		t.Fatal("initial paid interval was not committed")
	}
	policy.err = errors.New("current policy is unavailable")
	reads := policy.reads
	native.syncError = nil
	op, job = reloadOperation(t, s, op.Receipt.ID)
	if err := s.HandleProviderOperation(context.Background(), op, job); err != nil {
		t.Fatal(err)
	}
	if policy.reads != reads || balanceFixture(t, s, user) != paid {
		t.Fatal("accepted replay depended on mutable terms or charged again")
	}
	op, _ = reloadOperation(t, s, op.Receipt.ID)
	if op.Receipt.Status != "succeeded" {
		t.Fatal("application receipt did not complete")
	}
}

func TestHistoricalDisabledPaidActivationCannotResumeOwner(t *testing.T) {
	s, user, policy, native := commerceFixture(t)
	op, job := queuedActivation(t, s, user, policy)
	native.syncError = errNativeWait
	if err := s.HandleProviderOperation(context.Background(), op, job); err == nil {
		t.Fatal("expected native wait")
	}
	e, err := s.store.HourlyEnrollmentForActivation(context.Background(), user.ID, user.AccountGeneration, op.Receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.store.DisableHourlyEnrollment(context.Background(), database.HourlyDisableInput{OwnerID: user.ID, AccountGeneration: user.AccountGeneration, EnrollmentID: e.ID, Voluntary: true}, policy.snapshot, time.Now()); err != nil {
		t.Fatal(err)
	}
	before := native.resumes
	native.syncError = nil
	policy.err = errors.New("new terms unreadable")
	op, job = reloadOperation(t, s, op.Receipt.ID)
	if err = s.HandleProviderOperation(context.Background(), op, job); err != nil {
		t.Fatal(err)
	}
	if native.resumes != before || native.closes != 1 {
		t.Fatal("historical paid replay resumed access or skipped terminal cleanup")
	}
}

func TestKnownUnpaidFailureCancelsPreparationWithoutCooldown(t *testing.T) {
	s, user, policy, native := commerceFixture(t)
	op, job := queuedActivation(t, s, user, policy)
	native.preflightHook = func() {
		if _, err := s.store.AdjustBalance(context.Background(), user.ID, -99999, "spend", "fixture", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.HandleProviderOperation(context.Background(), op, job); err != nil {
		t.Fatal(err)
	}
	e, err := s.store.HourlyEnrollmentForActivation(context.Background(), user.ID, user.AccountGeneration, op.Receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != model.HourlyDisabled || e.ActivatedAt != nil || e.CooldownUntil != nil || native.resumes != 0 || native.closes != 1 || balanceFixture(t, s, user) != 1 {
		t.Fatal("unpaid preparation was charged, cooled down or enabled")
	}
	op, _ = reloadOperation(t, s, op.Receipt.ID)
	if op.Receipt.Status != "failed" || op.Receipt.ErrorCode == nil || *op.Receipt.ErrorCode != "INSUFFICIENT_BALANCE" {
		t.Fatal("known failure was not classified")
	}
}

func TestQueuedActivationCodeIsEncryptedAndChangedPayloadCannotReplay(t *testing.T) {
	s, user, policy, _ := commerceFixture(t)
	receipt, err := s.Activate(context.Background(), user, ActivationRequest{GroupID: "hourly-group", PolicyVersion: policy.snapshot.Version, OfferRevision: 1, ActivationCode: "private-code"}, "private-key")
	if err != nil {
		t.Fatal(err)
	}
	op, job := reloadOperation(t, s, receipt.ID)
	if strings.Contains(job.Payload, "private-code") || strings.Contains(op.RequestFingerprint, "private-code") {
		t.Fatal("activation code persisted in plaintext")
	}
	if _, err = s.Activate(context.Background(), user, ActivationRequest{GroupID: "different-group", PolicyVersion: policy.snapshot.Version, OfferRevision: 1}, "private-key"); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("changed idempotent request = %v", err)
	}
	if New(s.store, policy, nil, s.vault).native != nil {
		t.Fatal("typed nil native provider retained")
	}
}
