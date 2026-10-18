package rollover

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type purchaseRemote struct {
	Remote
	events          *[]string
	owner, purchase string
	err             error
}

func (r *purchaseRemote) QuiescePurchaseForRollover(_ context.Context, user model.User, purchase model.Purchase) error {
	*r.events = append(*r.events, "quiesce")
	r.owner, r.purchase = user.ID, purchase.ID
	return r.err
}
func (r *purchaseRemote) PurchaseUsageSnapshotForRollover(_ context.Context, user model.User, purchase model.Purchase, _ time.Time) (UsageSnapshot, error) {
	*r.events = append(*r.events, "usage")
	r.owner, r.purchase = user.ID, purchase.ID
	return UsageSnapshot{LimitBytes: 1000, Strategy: "NO_RESET", WeightedUsedBytes: 42, NodeSeriesAvailable: true, TermUsageExact: true}, r.err
}

func TestNativePurchaseSettlementHasNoLegacyIdentityGate(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		events := []string{}
		repo := &rolloverRepository{events: &events, eligible: true, rollover: model.PurchaseRollover{Status: "pending"}, purchase: model.Purchase{ID: "purchase", UserID: "member", ValidFrom: time.Now().Add(-time.Hour), ValidUntil: time.Now()}, user: model.User{ID: "member"}}
		remote := &purchaseRemote{events: &events}
		if blocked {
			remote.err = errors.New("native accounting not ready")
		}
		err := NewService(repo, remote).HandleOutbox(context.Background(), model.OutboxJob{Kind: "rollover_finalize", Payload: `{"purchaseId":"purchase"}`})
		if blocked {
			if err == nil || !reflect.DeepEqual(events, []string{"quiesce"}) {
				t.Fatal("unsettled native accounting was finalized", events, err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(events, []string{"quiesce", "mark", "usage", "calculated"}) || remote.owner != "member" || remote.purchase != "purchase" || repo.calculation.UsedBytes != 42 {
			t.Fatal("native settlement lost purchase identity", events, repo.calculation, err)
		}
	}
}
