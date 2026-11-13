package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestTemporaryBanCanRepeatWithIndependentExpiryAndFencedCompletion(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor, user := createTestAdmin(t, store, 48_231), createTestUser(t, store, 48_232)
	var previousUnban string
	for index, key := range []string{"first-ban", "second-ban"} {
		started := now.Add(time.Duration(index) * 10 * time.Minute)
		ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: user.ID,
			IdempotencyKey: key, RequestFingerprint: "temporary-ban:" + key, Reason: "investigation", DurationMinutes: 1}, started)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.BeginProviderOperationAttempt(ctx, ban.ID, started); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CompleteProviderOperation(ctx, ban.ID, providerops.Completion{Status: providerops.StatusSucceeded}, started); err != nil {
			t.Fatal(err)
		}
		if previousUnban != "" {
			if err := store.CompleteAdminTemporaryUnban(ctx, user.ID, previousUnban, started); !errors.Is(err, ErrConflict) {
				t.Fatalf("old unban cleared newer restriction: %v", err)
			}
		}
		if err := store.QueueExpiredAdminTemporaryBan(ctx, user.ID, started.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		active, err := store.ActiveAdminTemporaryBan(ctx, user.ID)
		if err != nil || active == nil || active.UnbanOperationID == "" || active.UnbanOperationID == previousUnban {
			t.Fatalf("independent expiry = %+v, %v", active, err)
		}
		if err := store.QueueExpiredAdminTemporaryBan(ctx, user.ID, started.Add(3*time.Minute)); err != nil {
			t.Fatalf("expiry retry changed fingerprint: %v", err)
		}
		unbanID := active.UnbanOperationID
		for range 2 {
			if err := store.CompleteAdminTemporaryUnban(ctx, user.ID, unbanID, started.Add(4*time.Minute)); err != nil {
				t.Fatalf("matching unban/replay failed: %v", err)
			}
		}
		previousUnban = unbanID
	}
	var bans, expiries, audits int
	for query, target := range map[string]*int{
		`SELECT COUNT(*) FROM provider_operations WHERE owner_user_id=$1 AND kind='admin_temporary_ban'`:   &bans,
		`SELECT COUNT(*) FROM provider_operations WHERE owner_user_id=$1 AND kind='admin_temporary_unban'`: &expiries,
		`SELECT COUNT(*) FROM audit_events WHERE target_id=$1 AND action='user.temporary_ban'`:             &audits,
	} {
		if err := store.DB().QueryRowContext(ctx, query, user.ID).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if bans != 2 || expiries != 2 || audits != 2 {
		t.Fatalf("prior evidence missing: bans=%d expiries=%d audits=%d", bans, expiries, audits)
	}
}

func TestManualUnbanKeepsReplacementExpiryIndependentlyScheduled(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor, user := createTestAdmin(t, store, 48_241), createTestUser(t, store, 48_242)
	createBan := func(key string, duration int, started time.Time) string {
		t.Helper()
		ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: user.ID,
			IdempotencyKey: key, RequestFingerprint: "temporary-ban:" + key, Reason: "investigation", DurationMinutes: duration}, started)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.BeginProviderOperationAttempt(ctx, ban.ID, started); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CompleteProviderOperation(ctx, ban.ID, providerops.Completion{Status: providerops.StatusSucceeded}, started); err != nil {
			t.Fatal(err)
		}
		return ban.ID
	}
	firstID := createBan("first-ban", 10, now)
	manual, err := store.CreateAdminTemporaryUnban(ctx, actor.ID, user.ID, "manual-release", "manual-release-fingerprint", "resolved", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteAdminTemporaryUnban(ctx, user.ID, manual.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	secondID := createBan("second-ban", 20, now.Add(2*time.Minute))
	rows, err := store.DB().QueryContext(ctx, `SELECT payload,available_at FROM outbox_jobs
		WHERE kind=$1 AND status='pending' ORDER BY available_at`, AdminTemporaryBanExpiryOutboxKind)
	if err != nil {
		t.Fatal(err)
	}
	type expiry struct{ payload, available string }
	jobs := []expiry{}
	for rows.Next() {
		var job expiry
		if err := rows.Scan(&job.payload, &job.available); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	if len(jobs) != 2 {
		t.Fatalf("replacement expiry was deduplicated with the old schedule: %+v", jobs)
	}
	for index, job := range jobs {
		var payload struct{ UserID, BanOperationID string }
		if err := json.Unmarshal([]byte(job.payload), &payload); err != nil || payload.UserID != user.ID || payload.BanOperationID != []string{firstID, secondID}[index] {
			t.Fatalf("expiry generation = %+v, %v", payload, err)
		}
		available, err := time.Parse(time.RFC3339Nano, job.available)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.QueueExpiredAdminTemporaryBan(ctx, payload.UserID, available); err != nil {
			t.Fatal(err)
		}
		active, err := store.ActiveAdminTemporaryBan(ctx, user.ID)
		if err != nil || active == nil || !active.ExpiresAt.Equal(now.Add(22*time.Minute)) || (active.UnbanOperationID != "") != (index == 1) {
			t.Fatalf("scheduled expiry did not preserve/release the replacement at its time: %+v, %v", active, err)
		}
	}
}
