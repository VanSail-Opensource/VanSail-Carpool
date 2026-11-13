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

func TestGatedNotificationsRejectDamagedFactsWithoutConsumingEvidence(t *testing.T) {
	t.Parallel()
	for index, kind := range []string{jobpayload.UserEventAutomaticReset, jobpayload.UserEventAddonActivated, jobpayload.UserEventMemberRefundCompleted} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 89110)
			other := createTestUser(t, store, 89111)
			now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
			key := "recovery:damaged:" + kind
			original, err := jobpayload.EncodeUserNotification(jobpayload.UserNotification{
				EventKey: key, UserID: user.ID, ChatID: user.TelegramID, Locale: "en", Kind: kind,
				OccurredAt: stamp(now), Facts: map[string]string{notifications.FactTime: stamp(now)},
			})
			if err != nil {
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
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			// This valid event sorts first, so a later invalid event must roll back
			// its attempted queue insertion too. Retained evidence is never edited.
			if _, err := store.insertUserNotificationTx(ctx, tx, "recovery:before:"+kind, user.ID, kind, userSyncGate(user.ID),
				map[string]string{notifications.FactTime: stamp(now)}, now); err != nil {
				t.Fatal(err)
			}
			// Model malformed historical/imported evidence at insertion; the
			// production immutability trigger must stay enabled throughout.
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_notification_events
				(event_key,user_id,source_kind,source_id,kind,payload_json,gate_key,created_at)
				VALUES($1,$2,'recovery','damaged',$3,$4,$5,$6)`, key, user.ID, kind, string(raw), userSyncGate(user.ID), stamp(now)); err != nil {
				t.Fatal(err)
			}
			if _, err := store.insertUserNotificationTx(ctx, tx, "recovery:independent:"+kind, other.ID, kind, userSyncGate(other.ID),
				map[string]string{notifications.FactTime: stamp(now)}, now); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			completed := now.Add(time.Minute)
			for range 2 {
				if err := store.ReleaseUserSyncNotifications(ctx, user.ID, completed); err == nil {
					t.Fatal("damaged gated notification was released")
				}
			}
			assertNotificationCounts(t, store, 3, 0)
			var pending int
			if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM user_notification_events WHERE user_id=$1 AND queued_at IS NULL`, user.ID).Scan(&pending); err != nil || pending != 2 {
				t.Fatalf("failed release consumed retained events: %d, %v", pending, err)
			}
			var retained string
			if err := store.DB().QueryRowContext(ctx, `SELECT payload_json FROM user_notification_events WHERE event_key=$1`, key).Scan(&retained); err != nil || retained != string(raw) {
				t.Fatal("failed release rewrote immutable evidence", err)
			}
			if err := store.ReleaseUserSyncNotifications(ctx, other.ID, completed); err != nil {
				t.Fatal(err)
			}
			if err := store.ReleaseUserSyncNotifications(ctx, other.ID, completed.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			assertNotificationCounts(t, store, 3, 1)
			var payload string
			if err := store.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind=$1`, jobpayload.UserNotificationKind).Scan(&payload); err != nil {
				t.Fatal(err)
			}
			decoded, err := jobpayload.DecodeUserNotification(model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: payload})
			if err != nil || decoded.UserID != other.ID || decoded.OccurredAt != stamp(completed) || decoded.Facts[notifications.FactTime] != stamp(completed) {
				t.Fatalf("independent notification = %+v, %v", decoded, err)
			}
		})
	}
}
