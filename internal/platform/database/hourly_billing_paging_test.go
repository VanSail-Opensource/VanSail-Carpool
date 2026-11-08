package database

import (
	"context"
	"testing"
	"time"
)

func TestHourlyDueOwnerCursorRotatesPastAnUnsettledFirstOwner(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	other := createTestUser(t, s, 89712)
	if _, err := s.AdjustBalance(ctx, other.ID, 100000, "other-hourly-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	second := input
	second.OwnerID, second.AccountGeneration, second.ActivationKey = other.ID, other.AccountGeneration, "other-owner"
	makeDueHourlyBillingFixture(t, s, second, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	first, err := s.DueHourlyOwnersPage(ctx, hourlyBillingNow(t, s), "", 1)
	if err != nil || len(first) != 1 {
		t.Fatalf("first due page = %+v %v", first, err)
	}
	next, err := s.DueHourlyOwnersPage(ctx, hourlyBillingNow(t, s), first[0].OwnerID, 1)
	if err != nil || len(next) != 1 || next[0].OwnerID == first[0].OwnerID {
		t.Fatalf("unsettled first owner starved next owner: %+v %v", next, err)
	}
	end, err := s.DueHourlyOwnersPage(ctx, hourlyBillingNow(t, s), next[0].OwnerID, 1)
	if err != nil || len(end) != 0 {
		t.Fatalf("cursor did not finish bounded cycle: %+v %v", end, err)
	}
}
