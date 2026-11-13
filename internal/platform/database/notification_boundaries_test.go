package database

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
)

func TestExpiryReminderPreservesExactWindow(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		offset time.Duration
		want   int
	}{
		{"last_nanosecond", time.Nanosecond, 1},
		{"expired", 0, 0},
		{"exact_window", 48 * time.Hour, 1},
		{"outside_window", 48*time.Hour + time.Nanosecond, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			owner := createTestUser(t, store, 88406)
			now := time.Now().UTC().Truncate(time.Second)
			combo := saveTestCombo(t, store, "Exact reminder", 100, 30)
			if _, err := store.AdjustBalance(ctx, owner.ID, 1000, "reminder-seed", "test", now); err != nil {
				t.Fatal(err)
			}
			purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, IdempotencyKey: "reminder"}, now.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',auto_renew_enabled=0,valid_until=$1 WHERE id=$2`, stamp(now.Add(scenario.offset)), purchase.ID); err != nil {
				t.Fatal(err)
			}
			if got, err := store.EnqueueExpiryReminderNotifications(ctx, now); err != nil || got != scenario.want {
				t.Fatal("exact reminder window differs", got, scenario.want, err)
			}
		})
	}
}

func TestExpirationNoticeSuppressesExactLaterSuccessor(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 88407)
	now := time.Now().UTC().Truncate(time.Second)
	combo := saveTestCombo(t, store, "Successor notice", 100, 30)
	if _, err := store.AdjustBalance(ctx, owner.ID, 1000, "successor-seed", "test", now); err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, IdempotencyKey: "first"}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, first.ID, true, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, IdempotencyKey: "second"}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired',auto_renew_enabled=0,valid_until=$1 WHERE id=$2`, stamp(now), first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='queued',valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(now), stamp(now.Add(time.Nanosecond)), second.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := store.insertExpirationNotificationTx(ctx, tx, first.ID, "", now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key=$1`, "expired:"+first.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("paid successor received a misleading expiration notice", count, err)
	}
}

func TestTrafficNoticeRearmsAtNanosecondReset(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	identity, period := notificationNativeUser(t, store, 88408)
	now := time.Now().UTC().Truncate(time.Second)
	user := notifications.TrafficUser{ID: identity.ProxyUserID, OwnerID: identity.UserID, PeriodID: period.ID}
	if _, err := store.DB().ExecContext(ctx, `UPDATE xnode_periods SET last_reset_at=$1 WHERE id=$2`, stamp(now.Add(-time.Hour)), period.ID); err != nil {
		t.Fatal(err)
	}
	if inserted, err := store.EnqueueTrafficThresholdNotification(ctx, user, now); err != nil || !inserted {
		t.Fatal(inserted, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE xnode_periods SET last_reset_at=$1 WHERE id=$2`, stamp(now.Add(time.Nanosecond)), period.ID); err != nil {
		t.Fatal(err)
	}
	if inserted, err := store.EnqueueTrafficThresholdNotification(ctx, user, now.Add(time.Second)); err != nil || !inserted {
		t.Fatal("previous period notice suppressed a new exact reset", inserted, err)
	}
}

func TestAdministratorExtensionNoticeKeepsLongDuration(t *testing.T) {
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	after := before.AddDate(2500, 0, 0)
	_, facts := adminEntitlementEditNotification(model.Purchase{ValidUntil: before}, AdminEntitlementEditInput{ValidUntil: after}, "", before)
	if facts[notifications.FactAddedSeconds] != strconv.FormatInt(after.Unix()-before.Unix(), 10) {
		t.Fatal("long extension notice saturated time.Duration", facts)
	}
}
