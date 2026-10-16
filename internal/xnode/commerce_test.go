package xnode

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type commerceRepository struct {
	Repository
	identity Identity
	period   Period
	periods  []Period
	purchase model.Purchase
	legacy   []LegacyHistory
}

func (r *commerceRepository) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	return []model.Purchase{r.purchase}, nil
}
func (r *commerceRepository) XNodeLegacyHistory(context.Context, int64, time.Time, time.Time) ([]LegacyHistory, error) {
	return r.legacy, nil
}

func TestRefundRejectsSourceTrafficBeforeTheImportedResetPeriod(t *testing.T) {
	r := &commerceRepository{identity: Identity{ProxyUserID: 42, UserID: "member", LegacyUserID: "7", CurrentPeriodID: "current"}, period: Period{ID: "current", PurchaseID: "purchase", Status: "open"}, purchase: model.Purchase{ID: "purchase"}, legacy: []LegacyHistory{{Date: "2026-10-01", WeightedBytes: 70}}}
	r.periods = []Period{r.period}
	s := &Service{repo: r}
	state, err := s.MemberRefundState(context.Background(), "42", "purchase")
	if err != nil || state.UsedTrafficBytes != 70 {
		t.Fatalf("source refund evidence was erased by reset: %+v,%v", state, err)
	}
}

func (r *commerceRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	return r.identity, nil
}
func (r *commerceRepository) XNodePeriod(context.Context, string) (Period, error) {
	return r.period, nil
}
func (r *commerceRepository) XNodePeriods(context.Context, int64) ([]Period, error) {
	return r.periods, nil
}
func (r *commerceRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return &r.purchase, nil
}
func (r *commerceRepository) SaveXNodePeriod(_ context.Context, p Period) error {
	r.period = p
	return nil
}

func TestMemberRefundIncludesEarlierResetsButExcludesAnotherPurchase(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		used         int64
		unavailable  bool
	}{
		{"settled", "closed", 7, false},
		{"unflushed", "closing", 7, true},
		{"review", "review", 7, true},
		{"overflow", "closed", math.MaxInt64, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &commerceRepository{identity: Identity{ProxyUserID: 42, CurrentPeriodID: "current"}, period: Period{ID: "current", PurchaseID: "purchase", Status: "open"}}
			r.periods = []Period{{ID: "old-reset", PurchaseID: "purchase", Status: tc.status, UsedBytes: tc.used}, {ID: "current", PurchaseID: "purchase", Status: "open", UsedBytes: 1}, {ID: "other", PurchaseID: "another", Status: "closed", UsedBytes: 900}}
			s := &Service{repo: r}
			state, err := s.MemberRefundState(context.Background(), "42", "purchase")
			if tc.unavailable {
				if !errors.Is(err, ErrUnavailable) {
					t.Fatalf("unsettled usage accepted: %v", err)
				}
				return
			}
			if err != nil || state.UsedTrafficBytes != 8 || state.Quiesced {
				t.Fatalf("refund state=%+v, %v", state, err)
			}
		})
	}
}

func TestApplyPurchaseUpdatesCadenceWithoutReplacingAccounting(t *testing.T) {
	until := time.Now().UTC().Add(24 * time.Hour)
	r := &commerceRepository{identity: Identity{ProxyUserID: 42, UserID: "member", CurrentPeriodID: "current"}, period: Period{ID: "current", PurchaseID: "purchase", Status: "open", Strategy: "DAY", UsedBytes: 17}, purchase: model.Purchase{ID: "purchase", TrafficLimitBytes: 1000, ResetStrategy: "WEEK", ValidUntil: until}}
	s := &Service{repo: r, now: time.Now}
	if err := s.ApplyPurchase(context.Background(), "42", 1000, "WEEK", until); err != nil {
		t.Fatal(err)
	}
	if r.period.Strategy != "WEEK" || r.period.UsedBytes != 17 || r.period.ID != "current" {
		t.Fatalf("cadence update replaced accounting: %+v", r.period)
	}
}
