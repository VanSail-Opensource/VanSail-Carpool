package rollover

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type periodRemote struct {
	rolloverRemote
	start, end time.Time
}

func (r *periodRemote) QuiesceRolloverPeriod(_ context.Context, _ string, start, end time.Time) error {
	*r.events = append(*r.events, "period")
	r.start, r.end = start, end
	return nil
}
func TestSettlementUsesPurchaseScopedQuiescence(t *testing.T) {
	events := []string{}
	id := "42"
	p := model.Purchase{ID: "old", ValidFrom: time.Now().UTC().Add(-48 * time.Hour), ValidUntil: time.Now().UTC().Add(-24 * time.Hour)}
	repo := &rolloverRepository{events: &events, eligible: true, rollover: model.PurchaseRollover{PurchaseID: p.ID, Status: "pending"}, purchase: p, user: model.User{ProxyUserID: &id}}
	remote := &periodRemote{rolloverRemote: rolloverRemote{events: &events}}
	if err := NewService(repo, remote).HandleOutbox(context.Background(), model.OutboxJob{Kind: "rollover_finalize", Payload: `{"purchaseId":"old"}`}); err != nil {
		t.Fatal(err)
	}
	if !remote.start.Equal(p.ValidFrom) || !remote.end.Equal(p.ValidUntil) || len(events) != 4 || events[0] != "period" {
		t.Fatalf("incorrect settlement scope: %v (%v,%v)", events, remote.start, remote.end)
	}
}
