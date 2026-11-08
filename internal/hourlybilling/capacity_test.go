package hourlybilling

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestGlobalCapacityRefusalCancelsUnpaidAuthorityWithoutCharge(t *testing.T) {
	s, user, policy, native := commerceFixture(t)
	op, job := queuedActivation(t, s, user, policy)
	ctx := context.Background()
	native.readyHook = func(context.Context, string) (xnode.HourlyReadiness, error) {
		return xnode.HourlyReadiness{Code: "XNODE_HOURLY_CAPACITY"}, nil
	}
	if !businessFailure(database.ErrHourlyCapacity) {
		t.Fatal("capacity was classified as indefinite dependency wait")
	}
	if err := s.HandleProviderOperation(ctx, op, job); err != nil {
		t.Fatal(err)
	}
	op, _ = reloadOperation(t, s, op.Receipt.ID)
	e, err := s.store.HourlyEnrollmentForActivation(ctx, user.ID, user.AccountGeneration, op.Receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Receipt.Status != "failed" || op.Receipt.ErrorCode == nil || *op.Receipt.ErrorCode != "HOURLY_CAPACITY" || e.Status != model.HourlyDisabled || e.ActivatedAt != nil || e.CooldownUntil != nil || native.resumes != 0 || native.closes != 1 || balanceFixture(t, s, user) != 100000 {
		t.Fatal("capacity refusal charged, resumed or cooled down unpaid access")
	}
}
