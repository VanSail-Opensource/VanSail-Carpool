package admin

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"testing"
	"time"
)

type refundUsageStub struct {
	snapshot rollover.UsageSnapshot
	err      error
	calls    int
	end      time.Time
}

func (r *refundUsageStub) UsageSnapshotForRollover(_ context.Context, _ string, _ time.Time, end time.Time) (rollover.UsageSnapshot, error) {
	r.calls++
	r.end = end
	return r.snapshot, r.err
}

func refundQuoteFixture(paid, limit, used int64, strategy string, days int) (*UserWorkflows, *refundUsageStub) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	remoteID := "1"
	repository := &userWorkflowRepositoryStub{user: model.User{ID: "user", RemnaUserID: &remoteID}, purchases: []model.Purchase{{ID: "purchase", UserID: "user", Status: "active", PriceTXBMinor: paid, GrossPriceTXBMinor: 10000, TrafficLimitBytes: limit, ResetStrategy: strategy, RolloverMinRemainingBPS: 8500, ValidFrom: start, ValidUntil: start.AddDate(0, 0, days)}}}
	current := int64(999999)
	source := &refundUsageStub{snapshot: rollover.UsageSnapshot{LimitBytes: 1, Strategy: "DAY", WeightedUsedBytes: used, NodeSeriesAvailable: true, CurrentUsedBytes: &current}}
	s := NewUserWorkflows(repository, nil)
	s.refundUsage = source
	s.now = func() time.Time { return start.Add(6 * 24 * time.Hour) }
	return s, source
}

func TestAdminRefundSuggestionUsesActualWholeTermTraffic(t *testing.T) {
	for _, test := range []struct {
		name, strategy    string
		paid, limit, used int64
		days              int
		want              string
	}{
		{"unused share below rollover threshold", "MONTH_ROLLING", 10000, 1000, 250, 30, "7500"},
		{"discounted debit", "MONTH_ROLLING", 7500, 1000, 250, 30, "5625"},
		{"custom traffic override", "MONTH_ROLLING", 10000, 2000, 500, 30, "7500"},
		{"multiple weekly periods", "WEEK", 10000, 1000, 1000, 28, "7500"},
		{"daily reset periods", "DAY", 12000, 1000, 1000, 30, "11600"},
		{"half cent rounds up", "MONTH_ROLLING", 101, 1000, 500, 30, "51"},
		{"allowance exhausted", "MONTH_ROLLING", 10000, 1000, 1000, 30, "0"},
		{"overused clamps to zero", "MONTH_ROLLING", 10000, 1000, 2000, 30, "0"},
		{"free entitlement", "MONTH_ROLLING", 0, 1000, 250, 30, "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, source := refundQuoteFixture(test.paid, test.limit, test.used, test.strategy, test.days)
			quote, err := s.RefundEntitlementQuote(context.Background(), "user", "purchase")
			if err != nil || quote.SuggestedRefund == nil || quote.SuggestedRefund.Minor != test.want {
				t.Fatalf("quote=%+v error=%v want=%s", quote, err, test.want)
			}
			if source.calls != 1 || !source.end.Equal(s.now()) {
				t.Fatalf("usage reads=%d end=%v", source.calls, source.end)
			}
		})
	}
}

func TestAdminRefundSuggestionQueuedAndUnavailable(t *testing.T) {
	s, source := refundQuoteFixture(10000, 1000, 250, "MONTH_ROLLING", 30)
	repository := s.repository.(*userWorkflowRepositoryStub)
	repository.purchases[0].Status = "queued"
	quote, err := s.RefundEntitlementQuote(context.Background(), "user", "purchase")
	if err != nil || quote.SuggestedRefund == nil || quote.SuggestedRefund.Minor != "10000" || source.calls != 0 {
		t.Fatalf("queued=%+v %v calls=%d", quote, err, source.calls)
	}
	repository.purchases[0].Status = "active"
	source.snapshot.NodeSeriesAvailable = false
	quote, err = s.RefundEntitlementQuote(context.Background(), "user", "purchase")
	if err != nil || quote.SuggestedRefund != nil || quote.ReasonCode == nil {
		t.Fatalf("missing usage=%+v %v", quote, err)
	}
	source.err = errors.New("provider outage")
	if _, err := s.RefundEntitlementQuote(context.Background(), "user", "purchase"); !errors.Is(err, ErrRefundUsageUnavailable) {
		t.Fatalf("provider outage=%v", err)
	}
	source.err = nil
	source.snapshot.NodeSeriesAvailable = true
	repository.user.RemnaUserID = nil
	quote, err = s.RefundEntitlementQuote(context.Background(), "user", "purchase")
	if err != nil || quote.SuggestedRefund != nil {
		t.Fatalf("unlinked=%+v %v", quote, err)
	}
	repository.purchases[0].UserID = "other"
	if _, err := s.RefundEntitlementQuote(context.Background(), "user", "purchase"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("ownership=%v", err)
	}
}
