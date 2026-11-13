package xnode

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type speedtestReadFailureRepository struct {
	Repository
	routeSpeedtestRepository
	failure     error
	starts      int
	completions []providerops.Completion
}

func (r *speedtestReadFailureRepository) XNodeIdentityByUser(context.Context, string) (Identity, error) {
	return Identity{}, r.failure
}
func (r *speedtestReadFailureRepository) BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error) {
	r.starts++
	return providerops.Item{Status: providerops.StatusProcessing}, nil
}
func (r *speedtestReadFailureRepository) CompleteProviderOperationItem(_ context.Context, _, _ string, completion providerops.Completion, _ time.Time) (providerops.Item, error) {
	r.completions = append(r.completions, completion)
	return providerops.Item{Status: completion.Status, ErrorCode: completion.ErrorCode}, nil
}

func TestRouteSpeedtestReadFailureDoesNotConsumeUnattemptedPair(t *testing.T) {
	for _, test := range []struct {
		name      string
		cause     error
		permanent bool
	}{
		{name: "storage outage", cause: errors.New("temporary storage failure")},
		{name: "deadline", cause: context.DeadlineExceeded},
		{name: "cancellation", cause: context.Canceled},
		{name: "node offline", cause: ErrUnavailable},
		{name: "capability pending", cause: &CapabilityError{Capability: "traffic_periods_v1"}},
		{name: "invalid access", cause: ErrInvalid, permanent: true},
		{name: "changed access", cause: ErrConflict, permanent: true},
		{name: "removed owner", cause: ErrNotFound, permanent: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &speedtestReadFailureRepository{failure: fmt.Errorf("load routing owner: %w", test.cause)}
			service := &Service{repo: repo, now: time.Now}
			op := providerops.Operation{Receipt: model.OperationReceipt{ID: "operation"}, OwnerUserID: "member"}
			item, err := service.runRouteSpeedtestPair(context.Background(), op, routeSpeedtestDocument{}, "00", RouteSpeedtestPair{})
			if !test.permanent {
				if !errors.Is(err, test.cause) || repo.starts != 0 || len(repo.completions) != 0 {
					t.Fatal("read failure consumed an unattempted pair", err, repo.starts, repo.completions)
				}
				return
			}
			if err != nil || repo.starts != 1 || len(repo.completions) != 1 || item.Status != providerops.StatusFailed || item.ErrorCode != "XNODE_SPEEDTEST_ACCESS_CHANGED" {
				t.Fatal("definitive access change lost its terminal outcome", item, err, repo.starts)
			}
		})
	}
}
