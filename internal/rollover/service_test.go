package rollover

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHandleOutboxCalculatesBeforeAutomaticRenewal(t *testing.T) {
	t.Parallel()
	transient := errors.New("temporary upstream failure")
	tests := []struct {
		name, status, exception string
		quiesceErr, usageErr    error
		wantEvents              []string
		wantError               bool
	}{
		{name: "happy", status: "pending", wantEvents: []string{"quiesce", "mark", "usage", "calculated"}},
		{name: "missing after quiesce", status: "pending", usageErr: ErrRemoteUserMissing, exception: "remnawave_user_missing", wantEvents: []string{"quiesce", "mark", "usage", "finalize"}},
		{name: "quiesce retry", status: "pending", quiesceErr: transient, wantEvents: []string{"quiesce"}, wantError: true},
		{name: "usage retry", status: "processing", usageErr: transient, wantEvents: []string{"usage"}, wantError: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			events := make([]string, 0, 4)
			remoteID := "remote-1"
			repository := &rolloverRepository{events: &events, eligible: true, rollover: model.PurchaseRollover{PurchaseID: "purchase-1", Status: test.status},
				purchase: model.Purchase{ID: "purchase-1", ValidFrom: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)}, user: model.User{ID: "user-1", RemnaUserID: &remoteID}}
			remote := &rolloverRemote{events: &events, quiesceErr: test.quiesceErr, usageErr: test.usageErr}
			service := NewService(repository, remote)
			service.now = func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }
			err := service.HandleOutbox(context.Background(), model.OutboxJob{Kind: "rollover_finalize", Payload: `{"purchaseId":"purchase-1"}`})
			if (err != nil) != test.wantError || repository.exception != test.exception {
				t.Fatalf("HandleOutbox() = (%v, %q), want error=%t exception=%q", err, repository.exception, test.wantError, test.exception)
			}
			if !reflect.DeepEqual(events, test.wantEvents) {
				t.Fatalf("events = %v, want %v", events, test.wantEvents)
			}
		})
	}
}

func TestHandleOutboxSkipsRemoteUsageWithoutAutoRenewalOrWithQueuedCombo(t *testing.T) {
	for _, eligible := range []bool{false} {
		events := make([]string, 0, 2)
		repository := &rolloverRepository{events: &events, eligible: eligible, rollover: model.PurchaseRollover{PurchaseID: "purchase-1", Status: "pending"}}
		if err := NewService(repository, &rolloverRemote{events: &events}).HandleOutbox(context.Background(), model.OutboxJob{Kind: "rollover_finalize", Payload: `{"purchaseId":"purchase-1"}`}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(events, []string{"mark", "finalize"}) {
			t.Fatalf("events=%v, want zero settlement without remote calls", events)
		}
	}
}

func TestHandleOutboxRejectsAggregateUsageFallback(t *testing.T) {
	events := make([]string, 0, 3)
	remoteID := "remote-1"
	repository := &rolloverRepository{events: &events, eligible: true, rollover: model.PurchaseRollover{PurchaseID: "purchase-1", Status: "processing"},
		purchase: model.Purchase{ID: "purchase-1", ValidUntil: time.Now().UTC()}, user: model.User{RemnaUserID: &remoteID}}
	err := NewService(repository, &rolloverRemote{events: &events, snapshot: UsageSnapshot{LimitBytes: 1_000}}).HandleOutbox(context.Background(), model.OutboxJob{Kind: "rollover_finalize", Payload: `{"purchaseId":"purchase-1"}`})
	if !errors.Is(err, ErrPerNodeUsageUnavailable) || !reflect.DeepEqual(events, []string{"usage"}) {
		t.Fatalf("aggregate fallback = (%v, %v)", err, events)
	}
}

type rolloverRepository struct {
	events      *[]string
	rollover    model.PurchaseRollover
	purchase    model.Purchase
	user        model.User
	eligible    bool
	exception   string
	calculation model.RolloverUsageSummary
}

func (r *rolloverRepository) RolloverByPurchase(context.Context, string) (model.PurchaseRollover, error) {
	return r.rollover, nil
}
func (r *rolloverRepository) UserForPurchase(context.Context, string) (model.User, error) {
	return r.user, nil
}
func (r *rolloverRepository) PurchaseByID(context.Context, string) (model.Purchase, error) {
	return r.purchase, nil
}
func (r *rolloverRepository) RolloverEligible(context.Context, string) (bool, error) {
	return r.eligible, nil
}
func (r *rolloverRepository) MarkRolloverProcessing(context.Context, string, time.Time) error {
	if r.rollover.Status == "pending" {
		*r.events = append(*r.events, "mark")
		r.rollover.Status = "processing"
	}
	return nil
}
func (r *rolloverRepository) RecordRolloverCalculation(_ context.Context, _ string, summary model.RolloverUsageSummary, _ time.Time) (model.PurchaseRollover, error) {
	*r.events = append(*r.events, "calculated")
	r.calculation, r.rollover.Status = summary, "calculated"
	return r.rollover, nil
}
func (r *rolloverRepository) FinalizeRollover(_ context.Context, _ string, _, _ int64, exception string, _ time.Time) (model.PurchaseRollover, error) {
	*r.events = append(*r.events, "finalize")
	r.exception = exception
	return r.rollover, nil
}

type rolloverRemote struct {
	events               *[]string
	quiesceErr, usageErr error
	snapshot             UsageSnapshot
}

func (r *rolloverRemote) QuiesceForRollover(context.Context, string) error {
	*r.events = append(*r.events, "quiesce")
	return r.quiesceErr
}
func (r *rolloverRemote) UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (UsageSnapshot, error) {
	*r.events = append(*r.events, "usage")
	if r.snapshot.LimitBytes == 0 {
		r.snapshot = UsageSnapshot{LimitBytes: 1_000, Strategy: "NO_RESET", NodeSeriesAvailable: true}
	}
	return r.snapshot, r.usageErr
}
