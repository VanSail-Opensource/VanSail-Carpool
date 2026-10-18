package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

type nativeProjectionRemote struct {
	*catalogRemnawave
	owner, purchase string
}

func (*nativeProjectionRemote) QuiescePurchaseForRollover(context.Context, model.User, model.Purchase) error {
	panic("projection must not quiesce")
}
func (r *nativeProjectionRemote) PurchaseUsageSnapshotForRollover(_ context.Context, user model.User, purchase model.Purchase, _ time.Time) (rollover.UsageSnapshot, error) {
	r.owner, r.purchase = user.ID, purchase.ID
	return rollover.UsageSnapshot{LimitBytes: 1000, Strategy: "NO_RESET", WeightedUsedBytes: 42, NodeSeriesAvailable: true, TermUsageExact: true}, nil
}

func TestProjectionReadsNativePurchaseWithoutLegacyIdentity(t *testing.T) {
	now := time.Now().UTC()
	purchase := model.Purchase{ID: "purchase", UserID: "member", Status: "active", AutoRenewEnabled: true, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}
	repo := &rolloverRepository{catalogRepository: &catalogRepository{}, purchase: purchase}
	remote := &nativeProjectionRemote{catalogRemnawave: &catalogRemnawave{}}
	service := NewService(repo, remote, time.Minute)
	service.now = func() time.Time { return now }
	projection, err := service.RolloverProjection(context.Background(), model.User{ID: "member", OnboardingState: "complete"}, purchase.ID)
	if err != nil || remote.owner != "member" || remote.purchase != purchase.ID || projection.ActualUsedTrafficBytes == nil || *projection.ActualUsedTrafficBytes != 42 {
		t.Fatal("native projection failed", projection, err)
	}
}
