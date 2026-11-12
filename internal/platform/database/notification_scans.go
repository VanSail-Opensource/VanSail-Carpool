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
func (s *Store) retryAttempt_EnqueueExpiryReminderNotifications(ctx context.Context, now time.Time) (int, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT purchases.id,purchases.user_id,combos.name,purchases.valid_until
		FROM purchases JOIN combos ON combos.id=purchases.combo_id
		WHERE purchases.status='active' AND purchases.valid_until>$1 AND purchases.valid_until<=$2
		AND purchases.auto_renew_enabled=0 AND NOT EXISTS (
			SELECT 1 FROM purchases queued WHERE queued.user_id=purchases.user_id AND queued.status='queued' AND queued.valid_until>$3
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

// EnqueueTrafficThresholdNotification rechecks the captured native lifecycle and
// current counters before recording one event per purchase reset period.
func (s *Store) retryAttempt_EnqueueTrafficThresholdNotification(ctx context.Context, user notifications.TrafficUser, now time.Time) (bool, error) {
	if user.ID <= 0 || user.OwnerID == "" || user.PeriodID == "" || user.AccountGeneration < 0 {
		return false, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var generation int64
	var state, onboarding string
	err = tx.QueryRowContext(ctx, `SELECT account_generation,account_status,onboarding_state
		FROM users WHERE id=$1 FOR SHARE`, user.OwnerID).Scan(&generation, &state, &onboarding)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if generation != user.AccountGeneration || state != "active" || onboarding != "complete" {
		return false, nil
	}
	var purchaseID, combo, validFrom, reset string
	var used, limit int64
	var lastReset sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT b.id,c.name,b.valid_from,p.strategy,p.used_bytes,p.limit_bytes,p.last_reset_at
		FROM xnode_identities i
		JOIN xnode_periods p ON p.id=i.current_period_id AND p.proxy_user_id=i.proxy_user_id
		JOIN purchases b ON b.id=p.purchase_id AND b.user_id=i.user_id
		JOIN combos c ON c.id=b.combo_id
		WHERE i.user_id=$1 AND i.proxy_user_id=$2 AND p.id=$3
		AND i.credentials_cipher<>'' AND i.access_disabled=0 AND p.status='open' AND b.status='active'
		AND b.valid_from::timestamptz<=CURRENT_TIMESTAMP AND b.valid_until::timestamptz>CURRENT_TIMESTAMP
		FOR SHARE OF i,p,b`, user.OwnerID, user.ID, user.PeriodID).Scan(
		&purchaseID, &combo, &validFrom, &reset, &used, &limit, &lastReset)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// This is floor(limit*9/10), decomposed to avoid overflowing int64.
	if used < 0 || limit <= 0 || used <= (limit/10)*9+(limit%10)*9/10 {
		return false, nil
	}
	anchor := validFrom
	if reset != "NO_RESET" && lastReset.Valid {
		at, err := parseStamp(lastReset.String)
		if err != nil {
			return false, err
		}
		anchor = stamp(at)
	}
	var alreadyQueued bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_notification_events
		WHERE source_kind='traffic-90' AND source_id=$1
		AND vansail_timestamp(created_at)>=vansail_timestamp($2))`, purchaseID, anchor).Scan(&alreadyQueued); err != nil {
		return false, err
	}
	if alreadyQueued {
		return false, nil
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	inserted, err := s.insertUserNotificationTx(ctx, tx, "traffic-90:"+purchaseID+":"+anchor, user.OwnerID,
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
