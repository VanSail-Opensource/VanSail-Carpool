package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestGatedNotificationsRejectDamagedFactsAndRemainRecoverable(t *testing.T) {
	t.Parallel()
	for index, kind := range []string{jobpayload.UserEventAutomaticReset, jobpayload.UserEventAddonActivated, jobpayload.UserEventMemberRefundCompleted} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 89110)
			now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
			key := "recovery:" + kind
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := store.insertUserNotificationTx(ctx, tx, key, user.ID, kind, userSyncGate(user.ID),
				map[string]string{notifications.FactTime: stamp(now)}, now); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			var original string
			if err := store.DB().QueryRowContext(ctx, `SELECT payload_json FROM user_notification_events WHERE event_key=$1`, key).Scan(&original); err != nil {
				t.Fatal(err)
			}
			var damaged map[string]any
			if err := json.Unmarshal([]byte(original), &damaged); err != nil {
				t.Fatal(err)
			}
			switch index {
			case 0:
				damaged["facts"] = nil
			case 1:
				delete(damaged, "facts")
			default:
				damaged["facts"] = map[string]string{}
			}
			raw, err := json.Marshal(damaged)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE user_notification_events SET payload_json=$1 WHERE event_key=$2`, string(raw), key); err != nil {
				t.Fatal(err)
			}
			completed := now.Add(time.Minute)
			if err := store.ReleaseUserSyncNotifications(ctx, user.ID, completed); err == nil {
				t.Fatal("damaged gated notification was released")
			}
			assertNotificationCounts(t, store, 1, 0)
			var pending bool
			if err := store.DB().QueryRowContext(ctx, `SELECT queued_at IS NULL FROM user_notification_events WHERE event_key=$1`, key).Scan(&pending); err != nil || !pending {
				t.Fatalf("failed release consumed recovery event: %t, %v", pending, err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE user_notification_events SET payload_json=$1 WHERE event_key=$2`, original, key); err != nil {
				t.Fatal(err)
			}
			if err := store.ReleaseUserSyncNotifications(ctx, user.ID, completed); err != nil {
				t.Fatal(err)
			}
			if err := store.ReleaseUserSyncNotifications(ctx, user.ID, completed.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			assertNotificationCounts(t, store, 1, 1)
			var payload string
			if err := store.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind=$1`, jobpayload.UserNotificationKind).Scan(&payload); err != nil {
				t.Fatal(err)
			}
			decoded, err := jobpayload.DecodeUserNotification(model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: payload})
			if err != nil || decoded.OccurredAt != stamp(completed) || decoded.Facts[notifications.FactTime] != stamp(completed) {
				t.Fatalf("repaired notification = %+v, %v", decoded, err)
			}
		})
	}
}
