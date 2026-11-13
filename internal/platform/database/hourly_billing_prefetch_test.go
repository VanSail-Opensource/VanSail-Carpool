package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// withHourlyPrefetchLead widens the bounded admission window for this sequential
// test only. Billing time is the PostgreSQL clock, so the window is the only
// deterministic way to exercise the pre-boundary interval. Every parallel test
// in this package remains paused until this test returns.
func withHourlyPrefetchLead(t *testing.T, lead time.Duration) {
	t.Helper()
	previous := hourlyPrefetchLead
	hourlyPrefetchLead = lead
	t.Cleanup(func() { hourlyPrefetchLead = previous })
}

// prefetchHourlyBillingFixture prepares an active enrollment whose paid boundary
// is a whole UTC hour at least one hour ahead, which is always inside the widened
// test window regardless of the minute in which the suite runs.
func prefetchHourlyBillingFixture(t *testing.T, s *Store, input HourlyActivationInput,
	snapshot model.HourlyPolicySnapshot) (model.HourlyEnrollment, time.Time) {
	t.Helper()
	prepared := makeDueHourlyBillingFixture(t, s, input, snapshot, hourlyBillingNow(t, s).Add(-4*time.Hour))
	boundary := hourlyBillingNow(t, s).Truncate(time.Hour).Add(2 * time.Hour)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET paid_through=$1 WHERE id=$2`, boundary, prepared.ID); err != nil {
		t.Fatal(err)
	}
	enrollment, err := s.HourlyEnrollment(context.Background(), prepared.ID)
	if err != nil {
		t.Fatal(err)
	}
	return enrollment, boundary
}

func hourlySyncJobCount(t *testing.T, s *Store, owner string) int {
	t.Helper()
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user'
		AND payload::jsonb->>'userId'=$1`, owner).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestHourlyPrefetchStartRequiresAnAlignedBoundaryInsideTheWindow(t *testing.T) {
	inside := time.Date(2026, 11, 10, 12, 59, 30, 0, time.UTC)
	edge := time.Date(2026, 11, 10, 12, 59, 0, 0, time.UTC)
	before := time.Date(2026, 11, 10, 12, 58, 59, 0, time.UTC)
	noon := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		paid     time.Time
		now      time.Time
		want     time.Time
		wantOkay bool
	}{
		{"inside window", noon.Add(time.Hour), inside, noon.Add(time.Hour), true},
		{"exactly at window edge", noon.Add(time.Hour), edge, noon.Add(time.Hour), true},
		{"outside window", noon.Add(time.Hour), before, time.Time{}, false},
		{"already expired", noon, noon.Add(30 * time.Minute), time.Time{}, false},
		{"not hour aligned", noon.Add(time.Hour + 30*time.Second), inside, time.Time{}, false},
		{"zero boundary", time.Time{}, inside, time.Time{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			start, okay := hourlyPrefetchStart(test.paid, test.now)
			if okay != test.wantOkay || !start.Equal(test.want) {
				t.Fatalf("prefetch start = %s, %t; want %s, %t", start, okay, test.want, test.wantOkay)
			}
		})
	}
}

func TestHourlyPrefetchBuysTheNextWholeHourBeforeTheBoundary(t *testing.T) {
	withHourlyPrefetchLead(t, 2*time.Hour)
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	enrollment, boundary := prefetchHourlyBillingFixture(t, s, input, policy)
	available := hourlyBillingBalance(t, s, input.OwnerID)
	result, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy,
		hourlyBillingDelivery(t, s, enrollment.ID), time.Time{})
	if err != nil || len(result.Settlements) != 1 || len(result.SkippedEnrollmentIDs) != 0 {
		t.Fatalf("prefetch settlement = %+v, %v", result, err)
	}
	receipt := result.Settlements[0]
	if !receipt.HourStart.Equal(boundary) || !receipt.ChargedFrom.Equal(boundary) ||
		!receipt.PaidUntil.Equal(boundary.Add(time.Hour)) || receipt.ChargedTXBMinor != 100 {
		t.Fatalf("prefetch interval = %+v", receipt)
	}
	// The debit is stamped at transaction time, never at the future interval.
	if !receipt.CreatedAt.Before(boundary) {
		t.Fatalf("prefetch receipt used a future debit time: %+v", receipt)
	}
	updated, err := s.HourlyEnrollment(ctx, enrollment.ID)
	if err != nil || updated.Status != model.HourlyActive || !updated.PaidThrough.Equal(boundary.Add(time.Hour)) {
		t.Fatalf("prefetch paid boundary = %+v, %v", updated, err)
	}
	if got := hourlyBillingBalance(t, s, input.OwnerID); got != available-100 {
		t.Fatalf("prefetch balance = %d, want %d", got, available-100)
	}
	if jobs := hourlySyncJobCount(t, s, input.OwnerID); jobs != 1 {
		t.Fatalf("prefetch queued %d native syncs, want exactly one", jobs)
	}
}

func TestHourlyConcurrentPrefetchChargesOneWholeHourOnce(t *testing.T) {
	withHourlyPrefetchLead(t, 2*time.Hour)
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	enrollment, boundary := prefetchHourlyBillingFixture(t, s, input, policy)
	available := hourlyBillingBalance(t, s, input.OwnerID)
	guards := hourlyBillingDelivery(t, s, enrollment.ID)
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy, guards, time.Time{})
			done <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	var receipts int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM hourly_settlements WHERE enrollment_id=$1`, enrollment.ID).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("concurrent prefetch receipts = %d, %v", receipts, err)
	}
	if got := hourlyBillingBalance(t, s, input.OwnerID); got != available-100 {
		t.Fatalf("concurrent prefetch debits = %d, want exactly one hour", available-got)
	}
	updated, err := s.HourlyEnrollment(ctx, enrollment.ID)
	if err != nil || !updated.PaidThrough.Equal(boundary.Add(time.Hour)) {
		t.Fatalf("concurrent prefetch boundary = %+v, %v", updated, err)
	}
}

func TestHourlyPrefetchInsufficientBalanceSkipsWithoutCuttingPaidAccess(t *testing.T) {
	withHourlyPrefetchLead(t, 2*time.Hour)
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	enrollment, boundary := prefetchHourlyBillingFixture(t, s, input, policy)
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=50 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	result, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy,
		hourlyBillingDelivery(t, s, enrollment.ID), time.Time{})
	if err != nil || len(result.Settlements) != 0 {
		t.Fatalf("unfunded prefetch settled = %+v, %v", result, err)
	}
	unchanged, err := s.HourlyEnrollment(ctx, enrollment.ID)
	if err != nil || unchanged.Status != model.HourlyActive || !unchanged.PaidThrough.Equal(boundary) {
		t.Fatalf("unfunded prefetch cancelled paid access: %+v, %v", unchanged, err)
	}
	var receipts, notifications int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM hourly_settlements WHERE enrollment_id=$1`, enrollment.ID).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("unfunded prefetch receipts = %d, %v", receipts, err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM user_notification_events`).Scan(&notifications); err != nil || notifications != 0 {
		t.Fatalf("unfunded prefetch emitted a definitive stop: %d, %v", notifications, err)
	}
	if got := hourlyBillingBalance(t, s, input.OwnerID); got != 50 {
		t.Fatalf("unfunded prefetch debited %d", 50-got)
	}
	// The skip keeps the prior state retryable: a later funded attempt still
	// buys exactly the next whole hour before the boundary passes.
	if _, err := s.AdjustBalance(ctx, input.OwnerID, 1000, "prefetch-topup", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	retried, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy,
		hourlyBillingDelivery(t, s, enrollment.ID), time.Time{})
	if err != nil || len(retried.Settlements) != 1 || !retried.Settlements[0].PaidUntil.Equal(boundary.Add(time.Hour)) {
		t.Fatalf("funded retry = %+v, %v", retried, err)
	}
	if got := hourlyBillingBalance(t, s, input.OwnerID); got != 950 {
		t.Fatalf("funded retry balance = %d, want 950", got)
	}
}

func TestHourlyDueOwnersAdmitOnlyFundedPrefetchWindows(t *testing.T) {
	withHourlyPrefetchLead(t, 2*time.Hour)
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	prefetchHourlyBillingFixture(t, s, input, policy)
	other := createTestUser(t, s, 89720)
	if _, err := s.AdjustBalance(ctx, other.ID, 100000, "other-prefetch-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	second := input
	second.OwnerID, second.AccountGeneration, second.ActivationKey = other.ID, other.AccountGeneration, "unfunded-prefetch"
	second.GroupID = "billing-hourly-b"
	addHourlyBillingOffer(t, s, hourlyBillingAdmin(t, s), second.GroupID, 100)
	prefetchHourlyBillingFixture(t, s, second, policy)
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=50 WHERE user_id=$1`, other.ID); err != nil {
		t.Fatal(err)
	}
	due, err := s.DueHourlyOwnersPage(ctx, hourlyBillingNow(t, s), "", 200)
	if err != nil {
		t.Fatal(err)
	}
	admitted := map[string]bool{}
	for _, owner := range due {
		admitted[owner.OwnerID] = true
	}
	if !admitted[input.OwnerID] || admitted[other.ID] {
		t.Fatalf("prefetch admission funded=%t unfunded=%t", admitted[input.OwnerID], admitted[other.ID])
	}
}
