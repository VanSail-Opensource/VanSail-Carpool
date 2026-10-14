package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func TestAbuseRecordCooldownAppliesToEveryAction(t *testing.T) {
	for _, action := range []string{"none", "warning", "ip_ban", "subscription_revoke", "temporary_ban"} {
		t.Run(action, func(t *testing.T) {
			store := newTestStore(t)
			user := createTestUser(t, store, 78_001)
			now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
			if _, err := store.DB().Exec(`UPDATE abuse_punishment_rules SET enabled=CASE WHEN action=? THEN 1 ELSE 0 END`, action); err != nil {
				t.Fatal(err)
			}
			policy := abuseCooldownPolicy(t, store)
			records, bans := 0, 0
			for _, step := range []struct {
				after time.Duration
				want  bool
			}{{0, true}, {time.Minute, false}, {30*time.Minute - time.Second, false}, {30 * time.Minute, true}} {
				at := now.Add(step.after)
				assertCooldownIncident(t, store, user.ID, policy, at, at, step.want)
				if step.want {
					records++
					if action == "temporary_ban" {
						bans++
					}
				}
				assertAbuseCooldownEffects(t, store, records, bans)
			}
			// Both emitted and suppressed incidents remain replay-safe after expiry.
			for _, bucket := range []time.Time{now, now.Add(time.Minute)} {
				assertCooldownIncident(t, store, user.ID, policy, bucket, now.Add(time.Hour), false)
			}
			assertAbuseCooldownEffects(t, store, records, bans)
			other := createTestUser(t, store, 78_004)
			assertCooldownIncident(t, store, other.ID, policy, now.Add(31*time.Minute), now.Add(31*time.Minute), true)
			policy.WarningCooldownMinutes = 0
			assertCooldownIncident(t, store, user.ID, policy, now.Add(31*time.Minute), now.Add(31*time.Minute), true)
			if action == "temporary_ban" {
				bans += 2
			}
			assertAbuseCooldownEffects(t, store, records+2, bans)
			var mismatched int
			if err := store.DB().QueryRow(`SELECT COUNT(*) FROM abuse_records WHERE selected_action<>?`, action).Scan(&mismatched); err != nil || mismatched != 0 {
				t.Fatalf("unexpected record actions = %d, error = %v", mismatched, err)
			}
		})
	}
}

func TestAbuseRecordCooldownDoesNotAdvanceEscalation(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 78_005)
	now := time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC)
	if _, err := store.DB().Exec(`UPDATE abuse_punishment_rules SET enabled=1,incident_threshold=CASE action WHEN 'warning' THEN 1 WHEN 'ip_ban' THEN 2 WHEN 'subscription_revoke' THEN 3 ELSE 4 END`); err != nil {
		t.Fatal(err)
	}
	policy := abuseCooldownPolicy(t, store)
	for index, action := range []string{"warning", "ip_ban", "subscription_revoke", "temporary_ban"} {
		at := now.Add(time.Duration(index) * 30 * time.Minute)
		assertCooldownIncident(t, store, user.ID, policy, at, at, true)
		var got string
		if err := store.DB().QueryRow(`SELECT selected_action FROM abuse_records WHERE user_id=? AND incident_bucket_at=?`, user.ID, stamp(at)).Scan(&got); err != nil || got != action {
			t.Fatalf("record %d action = %q, want %q, error = %v", index+1, got, action, err)
		}
		for _, delay := range []time.Duration{time.Minute, 2 * time.Minute, 29 * time.Minute} {
			assertCooldownIncident(t, store, user.ID, policy, at.Add(delay), at.Add(delay), false)
		}
		bans := 0
		if action == "temporary_ban" {
			bans = 1
		}
		assertAbuseCooldownEffects(t, store, index+1, bans)
	}
}

func TestAbuseRecordCooldownExpiryPrecision(t *testing.T) {
	store := newTestStore(t)
	policy := abuseCooldownPolicy(t, store)
	now := time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC)
	for index, step := range []struct {
		name  string
		after time.Duration
		want  bool
	}{
		{"just before expiry", 30*time.Minute - time.Nanosecond, false},
		{"exact expiry", 30 * time.Minute, true},
		{"just after expiry", 30*time.Minute + time.Nanosecond, true},
	} {
		t.Run(step.name, func(t *testing.T) {
			user := createTestUser(t, store, 79_010+int64(index))
			assertCooldownIncident(t, store, user.ID, policy, now, now, true)
			at := now.Add(step.after)
			assertCooldownIncident(t, store, user.ID, policy, at, at, step.want)
		})
	}
}

func TestAbuseProcessingCooldownPreservesReplayAndRollups(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 78_006)
	base := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	configureProcessingPolicy(t, store, 2, 2, base)
	for _, seconds := range []int{0, 1, 3, 4} {
		storeSecond(t, store, user.ID, base.Add(time.Duration(seconds)*time.Second), 2)
	}
	service := abuse.NewService(store, nil)
	if err := service.Process(ctx, base.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertIncidentFacts(t, store, user.ID, 2)
	assertAbuseCooldownEffects(t, store, 1, 0)
	if err := service.Process(ctx, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertAbuseCooldownEffects(t, store, 1, 0)
	var observations, pending int
	if err := store.DB().QueryRow(`SELECT SUM(observation_count) FROM abuse_qps_rollups`).Scan(&observations); err != nil || observations != 4 {
		t.Fatalf("QPS observations = %d, error = %v", observations, err)
	}
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM abuse_pending_log_events`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("pending events = %d, error = %v", pending, err)
	}
}

func abuseCooldownPolicy(t *testing.T, store *Store) abuse.Policy {
	t.Helper()
	policy, err := store.Policy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	policy.WarningCooldownMinutes = 30
	return policy
}

func assertCooldownIncident(t *testing.T, store *Store, userID string, policy abuse.Policy, bucket, now time.Time, want bool) {
	t.Helper()
	created, err := store.CreateIncident(context.Background(), userID, bucket, 10, 10, []string{"global"}, []string{"node-1"}, policy, now)
	if err != nil || created != want {
		t.Fatalf("CreateIncident(%s, %s) = (%t, %v), want (%t, nil)", bucket, now, created, err, want)
	}
}

func assertAbuseCooldownEffects(t *testing.T, store *Store, records, bans int) {
	t.Helper()
	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM abuse_records`:                               records,
		`SELECT COUNT(*) FROM abuse_temp_bans`:                             bans,
		`SELECT COUNT(*) FROM abuse_notification_deliveries`:               records,
		`SELECT COUNT(*) FROM outbox_jobs WHERE kind='abuse_punishment'`:   records,
		`SELECT COUNT(*) FROM outbox_jobs WHERE kind='abuse_notification'`: records,
	} {
		var got int
		if err := store.DB().QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s = %d, want %d, error = %v", query, got, want, err)
		}
	}
}
