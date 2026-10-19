package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

func TestRolloverProjectionValidatesOwnerAndUsesQueuedSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	remoteID := "remote-1"
	purchase := model.Purchase{ID: "purchase-1", UserID: "user-1", Status: "active", AutoRenewEnabled: true,
		PriceTXBMinor: 10_000, ValidFrom: now.Add(-24 * time.Hour), ValidUntil: now.Add(24 * time.Hour),
		Price: model.TXBMoney(10_000)}
	remote := &rolloverRemote{catalogRemnawave: &catalogRemnawave{}, snapshot: rollover.UsageSnapshot{LimitBytes: 1_000, Strategy: "NO_RESET", NodeSeriesAvailable: true}}
	repository := &rolloverRepository{catalogRepository: &catalogRepository{}, purchase: purchase}
	service := NewService(repository, remote, time.Minute)
	service.now = func() time.Time { return now }

	projection, err := service.RolloverProjection(context.Background(), model.User{ID: "user-1", OnboardingState: "complete", RemnaUserID: &remoteID}, purchase.ID)
	if err != nil || projection.PurchaseID != purchase.ID || !remote.start.Equal(purchase.ValidFrom) || !remote.end.Equal(now) {
		t.Fatalf("RolloverProjection() = (%+v, %v), range %s to %s", projection, err, remote.start, remote.end)
	}
	if _, err := service.RolloverProjection(context.Background(), model.User{ID: "other", OnboardingState: "complete", RemnaUserID: &remoteID}, purchase.ID); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("owner check error = %v", err)
	}
	if _, err := service.RolloverProjection(context.Background(), model.User{ID: "user-1", OnboardingState: "complete", RemnaUserID: &remoteID}, "missing"); err == nil {
		t.Fatal("missing purchase unexpectedly succeeded")
	}
}

func TestRolloverProjectionRejectsInactiveAndMissingIdentity(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	purchase := model.Purchase{ID: "purchase-1", UserID: "user-1", Status: "expired", AutoRenewEnabled: true, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}
	repository := &rolloverRepository{catalogRepository: &catalogRepository{}, purchase: purchase}
	service := NewService(repository, &rolloverRemote{catalogRemnawave: &catalogRemnawave{}}, time.Minute)
	service.now = func() time.Time { return now }
	if _, err := service.RolloverProjection(context.Background(), model.User{ID: "user-1", OnboardingState: "complete"}, purchase.ID); !errors.Is(err, ErrRolloverNotActive) {
		t.Fatalf("inactive error = %v", err)
	}
	purchase.Status = "active"
	repository.purchase = purchase
	if _, err := service.RolloverProjection(context.Background(), model.User{ID: "user-1", OnboardingState: "complete"}, purchase.ID); !errors.Is(err, ErrRolloverUnavailable) {
		t.Fatalf("missing identity error = %v", err)
	}
}

func TestRolloverProjectionPreservesIncompleteUsageCause(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	remoteID := "remote-1"
	purchase := model.Purchase{ID: "purchase-1", UserID: "user-1", Status: "active", AutoRenewEnabled: true,
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}
	repository := &rolloverRepository{catalogRepository: &catalogRepository{}, purchase: purchase}
	remote := &rolloverRemote{catalogRemnawave: &catalogRemnawave{}, err: rollover.ErrPerNodeUsageUnavailable}
	service := NewService(repository, remote, time.Minute)
	service.now = func() time.Time { return now }

	_, err := service.RolloverProjection(context.Background(), model.User{ID: "user-1", OnboardingState: "complete", RemnaUserID: &remoteID}, purchase.ID)
	if !errors.Is(err, ErrRolloverUnavailable) || !errors.Is(err, rollover.ErrPerNodeUsageUnavailable) {
		t.Fatalf("incomplete usage error = %v", err)
	}
}

func TestRolloverProjectionWarnsWithoutProviderUsageWhenAutoRenewalIsDisabled(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	remoteID := "remote-1"
	purchase := model.Purchase{ID: "purchase-disabled", UserID: "user-1", Status: "active", AutoRenewEnabled: false,
		PriceTXBMinor: 10_000, Price: model.TXBMoney(10_000), ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}
	remote := &rolloverRemote{catalogRemnawave: &catalogRemnawave{}}
	repository := &rolloverRepository{catalogRepository: &catalogRepository{}, purchase: purchase}
	service := NewService(repository, remote, time.Minute)
	service.now = func() time.Time { return now }

	projection, err := service.RolloverProjection(context.Background(), model.User{ID: "user-1", OnboardingState: "complete", RemnaUserID: &remoteID}, purchase.ID)
	if err != nil || projection.WarningCode == nil || *projection.WarningCode != "AUTO_RENEWAL_DISABLED" || remote.snapshotCalls != 0 {
		t.Fatalf("disabled rollover = (%+v, %v), snapshot calls=%d", projection, err, remote.snapshotCalls)
	}
}

type rolloverRepository struct {
	*catalogRepository
	purchase model.Purchase
}

func (r *rolloverRepository) PurchaseByID(_ context.Context, id string) (model.Purchase, error) {
	if r.purchase.ID == "" || id != r.purchase.ID {
		return model.Purchase{}, database.ErrNotFound
	}
	return r.purchase, nil
}

type rolloverRemote struct {
	*catalogRemnawave
	snapshot      rollover.UsageSnapshot
	err           error
	start         time.Time
	end           time.Time
	snapshotCalls int
}

func (r *rolloverRemote) UsageSnapshotForRollover(_ context.Context, _ string, start, end time.Time) (rollover.UsageSnapshot, error) {
	r.snapshotCalls++
	r.start, r.end = start, end
	return r.snapshot, r.err
}
