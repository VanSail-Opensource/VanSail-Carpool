package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// EnqueueExpiryReminderNotifications records newly eligible 48-hour reminders.
func (s *Store) EnqueueExpiryReminderNotifications(ctx context.Context, now time.Time) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT purchases.id,purchases.user_id,combos.name,purchases.valid_until
		FROM purchases JOIN combos ON combos.id=purchases.combo_id
		WHERE purchases.status='active' AND purchases.valid_until>? AND purchases.valid_until<=?
		AND purchases.auto_renew_enabled=0 AND NOT EXISTS (
			SELECT 1 FROM purchases queued WHERE queued.user_id=purchases.user_id AND queued.status='queued' AND queued.valid_until>?
		) ORDER BY purchases.valid_until,purchases.id`, stamp(now), stamp(now.Add(48*time.Hour)), stamp(now))
	if err != nil {
		return 0, err
	}
	type due struct{ purchaseID, userID, combo, expires string }
	items := make([]due, 0)
	for rows.Next() {
		var item due
		if err := rows.Scan(&item.purchaseID, &item.userID, &item.combo, &item.expires); err != nil {
			_ = rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	queued := 0
	for _, item := range items {
		inserted, err := s.insertUserNotificationTx(ctx, tx, "expiry-reminder:"+item.purchaseID+":"+item.expires,
			item.userID, jobpayload.UserEventExpiryReminder, "", map[string]string{
				notifications.FactCombo: item.combo, notifications.FactExpires: item.expires,
				notifications.FactAutoRenewal: "off", notifications.FactQueuedCombo: "none",
			}, now)
		if err != nil {
			return 0, err
		}
		if inserted {
			queued++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return queued, nil
}

// EnqueueTrafficThresholdNotification records one event per purchase reset period.
func (s *Store) EnqueueTrafficThresholdNotification(ctx context.Context, remoteID string, used, limit int64,
	remoteReset string, lastReset *time.Time, now time.Time) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var purchaseID, userID, combo, validFrom, reset string
	err = tx.QueryRowContext(ctx, `SELECT purchases.id,purchases.user_id,combos.name,purchases.valid_from,
		COALESCE(purchases.entitlement_reset_strategy,combos.reset_strategy)
		FROM users JOIN purchases ON purchases.user_id=users.id JOIN combos ON combos.id=purchases.combo_id
		WHERE users.proxy_user_id=? AND purchases.status='active'
		AND purchases.valid_from<=? AND purchases.valid_until>? ORDER BY purchases.valid_from DESC LIMIT 1`,
		remoteID, stamp(now), stamp(now)).Scan(&purchaseID, &userID, &combo, &validFrom, &reset)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if remoteReset != "" {
		reset = remoteReset
	}
	anchor := validFrom
	if reset != "NO_RESET" && lastReset != nil {
		anchor = stamp(lastReset.UTC())
	}
	var alreadyQueued int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_notification_events
		WHERE event_key LIKE ? AND created_at>=?)`, "traffic-90:"+purchaseID+":%", anchor).Scan(&alreadyQueued); err != nil {
		return false, err
	}
	if alreadyQueued == 1 {
		return false, nil
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	inserted, err := s.insertUserNotificationTx(ctx, tx, "traffic-90:"+purchaseID+":"+anchor, userID,
		jobpayload.UserEventTrafficThreshold, "", map[string]string{
			notifications.FactCombo: combo, notifications.FactUsed: strconv.FormatInt(used, 10),
			notifications.FactTrafficLimit: strconv.FormatInt(limit, 10),
			notifications.FactRemaining:    strconv.FormatInt(remaining, 10), notifications.FactReset: reset,
		}, now)
	if err != nil {
		return false, fmt.Errorf("queue traffic notification: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return inserted, nil
}
