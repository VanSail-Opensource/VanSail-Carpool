package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestOldPurchaseCannotResetOrQuiesceItsSuccessor(t *testing.T) {
	r := &commerceRepository{identity: Identity{ProxyUserID: 42, UserID: "member", CurrentPeriodID: "next"}, period: Period{ID: "next", ProxyUserID: 42, PurchaseID: "new", Status: "closing", UsedBytes: 77}, purchase: model.Purchase{ID: "new", UserID: "member"}}
	s := &Service{repo: r, now: time.Now}
	for _, action := range []func() error{
		func() error { return s.ResetPurchaseTraffic(context.Background(), "42", "old") },
		func() error { return s.QuiescePurchaseOperation(context.Background(), "42", "old") },
		func() error { _, err := s.MemberRefundState(context.Background(), "42", "old"); return err },
	} {
		if err := action(); !errors.Is(err, ErrConflict) {
			t.Fatal("old purchase escaped its scope", err)
		}
		if r.period.Status != "closing" || r.period.UsedBytes != 77 || r.identity.CurrentPeriodID != "next" {
			t.Fatal("successor was changed", r.period, r.identity)
		}
	}
}

type resetBoundaryRepository struct {
	commerceRepository
	reads int
	saves int
}

func (r *resetBoundaryRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	r.reads++
	id := "old"
	if r.reads > 1 {
		id = "new"
	}
	return &model.Purchase{ID: id, UserID: "member"}, nil
}
func (r *resetBoundaryRepository) SaveXNodePeriod(context.Context, Period) error {
	r.saves++
	return nil
}

func TestResetRechecksThePurchaseAfterItsAccountingBarrier(t *testing.T) {
	r := &resetBoundaryRepository{commerceRepository: commerceRepository{identity: Identity{ProxyUserID: 42, UserID: "member", CurrentPeriodID: "old-period"}, period: Period{ID: "old-period", ProxyUserID: 42, PurchaseID: "old", Status: "closed"}}}
	s := &Service{repo: r, now: time.Now}
	if err := s.ResetPurchaseTraffic(context.Background(), "42", "old"); !errors.Is(err, ErrConflict) || r.saves != 0 {
		t.Fatal("reset opened a stale or successor period", err, r.saves)
	}
}

type resetProofRepository struct {
	commerceRepository
	operation Period
}

func (r *resetProofRepository) XNodePeriod(_ context.Context, id string) (Period, error) {
	if id == r.operation.ID {
		return r.operation, nil
	}
	if id == r.period.ID {
		return r.period, nil
	}
	return Period{}, ErrNotFound
}

func TestResetOperationProofSurvivesAStartedSuccessor(t *testing.T) {
	const operation = "550e8400-e29b-41d4-a716-446655440000"
	at := time.Now().Add(-time.Hour)
	r := &resetProofRepository{commerceRepository: commerceRepository{identity: Identity{ProxyUserID: 42, UserID: "member", CurrentPeriodID: "next"}, period: Period{ID: "next", ProxyUserID: 42, PurchaseID: "new", Status: "open", UsedBytes: 77}}, operation: Period{ID: operation, ProxyUserID: 42, PurchaseID: "old", Status: "closed", LastResetAt: &at}}
	s := &Service{repo: r, now: time.Now}
	if done, err := s.ResetOperationCompleted(context.Background(), "42", "old", operation); err != nil || !done {
		t.Fatal(done, err)
	}
	if err := s.ResetPurchaseTrafficForOperation(context.Background(), "42", "old", operation); err != nil {
		t.Fatal("confirmed reset was repeated or rejected", err)
	}
	if r.period.UsedBytes != 77 || r.period.Status != "open" || r.identity.CurrentPeriodID != "next" {
		t.Fatal("proof replay disturbed its successor", r.period)
	}
	if _, err := s.ResetOperationCompleted(context.Background(), "42", "different", operation); !errors.Is(err, ErrConflict) {
		t.Fatal("proof escaped purchase ownership", err)
	}
}
