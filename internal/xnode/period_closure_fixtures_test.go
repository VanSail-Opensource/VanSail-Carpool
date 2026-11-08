package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Historical-term fixtures model the same generation check as persistent
// repositories. Live blocked barriers use the hosted control-plane fixture.
func (r *commerceRepository) BeginXNodePeriodClosure(_ context.Context, expected Period, _ time.Time) (Period, error) {
	if expected.Status != r.period.Status || !samePeriodClosure(expected, r.period) {
		return Period{}, ErrConflict
	}
	return r.period, nil
}

func (r *termRepository) BeginXNodePeriodClosure(_ context.Context, expected Period, _ time.Time) (Period, error) {
	for _, current := range r.periods {
		if samePeriodClosure(current, expected) && current.Status == expected.Status {
			return current, nil
		}
	}
	return Period{}, ErrConflict
}

type reopenedClosedRepository struct {
	commerceRepository
	checks, saves int
}

func (r *reopenedClosedRepository) BeginXNodePeriodClosure(_ context.Context, _ Period, _ time.Time) (Period, error) {
	r.checks++
	r.period.Status = "open" // Another mutation reopened the captured closed term.
	return Period{}, ErrConflict
}

func (r *reopenedClosedRepository) SaveXNodePeriod(context.Context, Period) error {
	r.saves++
	return nil
}

func TestClosedSnapshotCannotBypassReopenedAdmissionFence(t *testing.T) {
	r := &reopenedClosedRepository{commerceRepository: commerceRepository{
		identity: Identity{ProxyUserID: 42, UserID: "member", CurrentPeriodID: "period"},
		period:   Period{ID: "period", ProxyUserID: 42, PurchaseID: "purchase", Status: "closed", ClosureGeneration: 7},
		purchase: model.Purchase{ID: "purchase", UserID: "member"},
	}}
	s := &Service{repo: r, now: time.Now}
	err := s.ResetPurchaseTraffic(context.Background(), "42", "purchase")
	if !errors.Is(err, ErrConflict) || r.checks != 1 || r.saves != 0 || r.period.Status != "open" {
		t.Fatal("stale closed snapshot admitted a successor", err, r.checks, r.saves, r.period)
	}
}

func TestQuiescedRefundRejectsAnotherUnfinishedPeriodInSamePurchase(t *testing.T) {
	r := &commerceRepository{
		identity: Identity{ProxyUserID: 42, UserID: "member", CurrentPeriodID: "closed-current"},
		period:   Period{ID: "closed-current", ProxyUserID: 42, PurchaseID: "purchase", Status: "closed", ClosureGeneration: 2},
		purchase: model.Purchase{ID: "purchase", UserID: "member"},
	}
	r.periods = []Period{r.period, {ID: "old-unfinished", ProxyUserID: 42, PurchaseID: "purchase", Status: "open"}}
	s := &Service{repo: r, now: time.Now}
	if _, err := s.MemberRefundState(context.Background(), "42", "purchase"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("zero-byte unfinished accounting qualified for refund", err)
	}
}
