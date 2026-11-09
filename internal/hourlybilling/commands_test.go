package hourlybilling

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestDisableForAdminQueuesDistinctActorAndStopsWithoutCooldown(t *testing.T) {
	s, user, policy, _ := commerceFixture(t)
	ctx := context.Background()
	admin, _, err := s.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 97_825, FirstName: "Admin"}, false)
	if err != nil {
		t.Fatal(err)
	}
	activation, activationJob := queuedActivation(t, s, user, policy)
	if err := s.HandleProviderOperation(ctx, activation, activationJob); err != nil {
		t.Fatalf("activation = %v", err)
	}
	enrollment, err := s.store.HourlyEnrollmentForActivation(ctx, user.ID, user.AccountGeneration, activation.Receipt.ID)
	if err != nil || enrollment.Status != model.HourlyActive {
		t.Fatalf("enrollment = %+v, %v", enrollment, err)
	}
	attacker, _, err := s.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 97_826, FirstName: "Other"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Disable(ctx, attacker, enrollment.ID, policy.snapshot.Version, "member-stop"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("member stop for another owner = %v", err)
	}

	receipt, err := s.DisableForAdmin(ctx, admin.ID, user.ID, enrollment.ID, policy.snapshot.Version, "admin-stop")
	if err != nil {
		t.Fatalf("DisableForAdmin(): %v", err)
	}
	queued, err := s.store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if queued.ActorUserID != admin.ID || queued.OwnerUserID != user.ID ||
		queued.Receipt.Kind != DisableOperation || queued.Receipt.Status != "queued" {
		t.Fatalf("admin operation = %+v", queued)
	}
	replay, err := s.DisableForAdmin(ctx, admin.ID, user.ID, enrollment.ID, policy.snapshot.Version, "admin-stop")
	if err != nil || replay.ID != receipt.ID {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	if _, err := s.DisableForAdmin(ctx, admin.ID, user.ID, enrollment.ID,
		strings.Repeat("a", 64), "stale-admin-stop"); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("stale policy stop = %v", err)
	}

	operation, job := reloadOperation(t, s, receipt.ID)
	if err := s.HandleProviderOperation(ctx, operation, job); err != nil {
		t.Fatalf("admin disable = %v", err)
	}
	stopped, err := s.store.HourlyEnrollment(ctx, enrollment.ID)
	if err != nil || stopped.Status != model.HourlyDisabled || stopped.CooldownUntil != nil {
		t.Fatalf("stopped enrollment = %+v, %v", stopped, err)
	}
	if stopped.PaidThrough == nil || enrollment.PaidThrough == nil || !stopped.PaidThrough.Equal(*enrollment.PaidThrough) {
		t.Fatal("administrator disable did not retain the paid hour")
	}
	if _, err := s.DisableForAdmin(ctx, admin.ID, user.ID, enrollment.ID,
		policy.snapshot.Version, "terminated-admin-stop"); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("terminated enrollment stop = %v", err)
	}
	operation, _ = reloadOperation(t, s, receipt.ID)
	if operation.Receipt.Status != "succeeded" {
		t.Fatalf("admin receipt = %+v", operation.Receipt)
	}
}
