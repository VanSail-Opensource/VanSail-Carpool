package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func TestAbuseProcessingContinuesAcrossTasksAndEmitsOnce(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 78_101)
	service := abuse.NewService(store, nil)
	base := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	configureProcessingPolicy(t, store, 2, 2, base)
	storeSecond(t, store, user.ID, base, 2)
	if err := service.Process(ctx, base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 0)
	storeSecond(t, store, user.ID, base.Add(time.Second), 2)
	if err := service.Process(ctx, base.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 1)
	storeSecond(t, store, user.ID, base.Add(2*time.Second), 2)
	if err := service.Process(ctx, base.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 1)
}

func TestAbuseProcessingGapResetsStreak(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 78_102)
	service := abuse.NewService(store, nil)
	base := time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)
	configureProcessingPolicy(t, store, 2, 2, base)
	storeSecond(t, store, user.ID, base, 2)
	storeSecond(t, store, user.ID, base.Add(time.Second), 2)
	storeSecond(t, store, user.ID, base.Add(3*time.Second), 2)
	storeSecond(t, store, user.ID, base.Add(4*time.Second), 2)
	if err := service.Process(ctx, base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 2)
}

func TestAbuseProcessingRecoversClaimsAndWritesRollups(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 78_103)
	service := abuse.NewService(store, nil)
	base := time.Date(2026, 8, 24, 14, 0, 0, 0, time.UTC)
	configureProcessingPolicy(t, store, 1, 1, base)
	storeSecond(t, store, user.ID, base, 3)
	claim, err := store.ClaimEvents(ctx, base.Add(time.Minute), base.Add(time.Minute), 50)
	if err != nil || len(claim.Events) != 3 {
		t.Fatalf("ClaimEvents() = %d, %v", len(claim.Events), err)
	}
	if err = service.RecoverProcessing(ctx); err != nil {
		t.Fatal(err)
	}
	if err = service.Process(ctx, base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 1)
	var observations, sum, minimum, maximum int
	err = store.DB().QueryRowContext(ctx, `SELECT observation_count,qps_sum,qps_min,qps_max FROM abuse_qps_rollups WHERE window_at=?`, stamp(base)).Scan(&observations, &sum, &minimum, &maximum)
	if err != nil || observations != 1 || sum != 3 || minimum != 3 || maximum != 3 {
		t.Fatalf("rollup = (%d,%d,%d,%d,%v)", observations, sum, minimum, maximum, err)
	}
}

func TestAbuseProcessingClaimDoesNotSplitUserSecondBucket(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 78_105)
	base := time.Date(2026, 8, 24, 14, 30, 0, 0, time.UTC)
	storeSecond(t, store, user.ID, base, 2)
	storeSecond(t, store, user.ID, base.Add(time.Second), 1)

	claim, err := store.ClaimEvents(ctx, base.Add(time.Minute), base.Add(time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(claim.Events) != 2 {
		t.Fatalf("ClaimEvents() returned %d events, want complete two-event bucket", len(claim.Events))
	}
	for _, event := range claim.Events {
		if !event.EventSecond.Equal(base) {
			t.Fatalf("ClaimEvents() included event at %s beyond boundary %s", event.EventSecond, base)
		}
	}
}

func TestAbuseProcessingUsesLatestPolicyAndBelowLimitResets(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 78_104)
	service := abuse.NewService(store, nil)
	base := time.Date(2026, 8, 24, 15, 0, 0, 0, time.UTC)
	configureProcessingPolicy(t, store, 2, 3, base)
	storeSecond(t, store, user.ID, base, 2)
	storeSecond(t, store, user.ID, base.Add(time.Second), 2)
	if err := service.Process(ctx, base.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 0)
	configureProcessingPolicy(t, store, 2, 2, base.Add(7*time.Minute))
	storeSecond(t, store, user.ID, base.Add(2*time.Second), 2)
	if err := service.Process(ctx, base.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 1)
	configureProcessingPolicy(t, store, 3, 2, base.Add(9*time.Minute))
	storeSecond(t, store, user.ID, base.Add(3*time.Second), 2)
	storeSecond(t, store, user.ID, base.Add(4*time.Second), 3)
	storeSecond(t, store, user.ID, base.Add(5*time.Second), 3)
	if err := service.Process(ctx, base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 2)
}

func configureProcessingPolicy(t *testing.T, store *Store, limit, streak int, now time.Time) {
	t.Helper()
	policy, err := store.Policy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	policy.GlobalEnabled, policy.GlobalLimit, policy.StreakSeconds = true, limit, streak
	if _, err = store.UpdatePolicy(context.Background(), "admin", policy, now); err != nil {
		t.Fatal(err)
	}
}

func storeSecond(t *testing.T, store *Store, userID string, at time.Time, count int) {
	t.Helper()
	events := make([]abuse.LogEvent, 0, count)
	for index := 0; index < count; index++ {
		events = append(events, abuse.LogEvent{UserID: userID, NodeUUID: "node-1", Domain: "example.com", Fingerprint: fmt.Sprintf("%s-%d", at.Format(time.RFC3339Nano), index), EventSecond: at})
	}
	if _, err := store.StoreEvents(context.Background(), "node-1", events, abuse.ReportCounts{}, at); err != nil {
		t.Fatal(err)
	}
}

func assertIncidentFacts(t *testing.T, store *Store, userID string, want int) {
	t.Helper()
	var got int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM abuse_incident_facts WHERE user_id=?`, userID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("incident facts = %d, want %d", got, want)
	}
}
