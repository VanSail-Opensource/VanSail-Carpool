package xnode

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type termRepository struct {
	Repository
	identity Identity
	periods  []Period
	purchase model.Purchase
	native   []DailyUsage
	legacy   []LegacyHistory
}

func (r *termRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	return r.identity, nil
}
func (r *termRepository) XNodePeriods(context.Context, int64) ([]Period, error) {
	return r.periods, nil
}
func (r *termRepository) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	return []model.Purchase{r.purchase}, nil
}
func (r *termRepository) XNodeUsage(context.Context, int64, time.Time, time.Time) ([]DailyUsage, error) {
	return r.native, nil
}
func (r *termRepository) XNodeLegacyHistory(context.Context, int64, time.Time, time.Time) ([]LegacyHistory, error) {
	return r.legacy, nil
}

func termFixture() (*Service, *termRepository) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	r := &termRepository{identity: Identity{UserID: "member", ProxyUserID: 42, CurrentPeriodID: "successor"}, purchase: model.Purchase{ID: "old", ValidFrom: start, ValidUntil: start.AddDate(0, 0, 2)}}
	r.periods = []Period{{ID: "old-period", PurchaseID: "old", StartedAt: start, Status: "closed", UsedBytes: 50, LimitBytes: 1000, Strategy: "NO_RESET"}, {ID: "successor", PurchaseID: "new", StartedAt: r.purchase.ValidUntil, Status: "open"}}
	r.native = []DailyUsage{{PeriodID: "old-period", Date: "2026-10-01", WeightedBytes: 20}, {PeriodID: "successor", Date: "2026-10-01", WeightedBytes: 900}}
	r.legacy = []LegacyHistory{{Date: "2026-10-01", WeightedBytes: 30}, {Date: "2026-10-03", WeightedBytes: 700}}
	return &Service{repo: r}, r
}

func TestRolloverCombinesLegacyHistoryWithoutIncludingSuccessor(t *testing.T) {
	s, r := termFixture()
	value, err := s.UsageSnapshotForRollover(context.Background(), "42", r.purchase.ValidFrom, r.purchase.ValidUntil)
	if err != nil || !value.NodeSeriesAvailable || value.WeightedUsedBytes != 50 || len(value.Daily) != 1 || value.Daily[0].Bytes != 50 || value.CurrentUsedBytes == nil || *value.CurrentUsedBytes != 50 {
		t.Fatalf("combined snapshot=%+v,%v", value, err)
	}
	// The successor remains open. Replayed old-term quiescence never selects it.
	if err = s.QuiesceRolloverPeriod(context.Background(), "42", r.purchase.ValidFrom, r.purchase.ValidUntil); err != nil || r.periods[1].Status != "open" {
		t.Fatalf("old-term quiescence interrupted successor: %v", err)
	}
}

func TestRolloverRejectsPendingJournalsAndCounterOverflow(t *testing.T) {
	s, r := termFixture()
	r.periods[0].Status = "closing"
	if _, err := s.UsageSnapshotForRollover(context.Background(), "42", r.purchase.ValidFrom, r.purchase.ValidUntil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("pending journal accepted: %v", err)
	}
	r.periods[0].Status = "closed"
	r.legacy[0].WeightedBytes = math.MaxInt64
	if _, err := s.UsageSnapshotForRollover(context.Background(), "42", r.purchase.ValidFrom, r.purchase.ValidUntil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("overflow accepted: %v", err)
	}
}
