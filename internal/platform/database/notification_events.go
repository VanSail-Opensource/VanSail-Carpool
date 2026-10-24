package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func (s *Store) insertUserNotificationTx(ctx context.Context, tx *sql.Tx, eventKey, userID, kind, gateKey string,
	facts map[string]string, now time.Time) (bool, error) {
	eventKey, userID, gateKey = strings.TrimSpace(eventKey), strings.TrimSpace(userID), strings.TrimSpace(gateKey)
	sourceKind, sourceID, err := notificationSource(eventKey)
	if err != nil {
		return false, err
	}
	var chatID int64
	var locale string
	if err := tx.QueryRowContext(ctx, `SELECT telegram_id,notification_locale FROM users WHERE id=?`, userID).Scan(&chatID, &locale); err != nil {
		return false, fmt.Errorf("load notification recipient: %w", err)
	}
	payload, err := jobpayload.EncodeUserNotification(jobpayload.UserNotification{
		EventKey: eventKey, UserID: userID, ChatID: chatID, Locale: locale, Kind: kind,
		OccurredAt: now.UTC().Format(time.RFC3339Nano), Facts: facts,
	})
	if err != nil {
		return false, err
	}
	var queuedAt any
	if gateKey == "" {
		queuedAt = stamp(now)
	}
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO user_notification_events
		(event_key,user_id,source_kind,source_id,kind,payload_json,gate_key,queued_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		eventKey, userID, sourceKind, sourceID, kind, payload, gateKey, queuedAt, stamp(now))
	if err != nil {
		return false, fmt.Errorf("insert user notification event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		if s.logger != nil {
			s.logger.Debug("user notification duplicate suppressed", "event_key", eventKey, "kind", kind, "user_id", userID)
		}
		return false, nil
	}
	if gateKey == "" {
		if err := insertOutboxTx(ctx, tx, jobpayload.UserNotificationKind, payload, now, now); err != nil {
			return false, err
		}
	}
	if s.logger != nil {
		s.logger.Info("user notification event created", "event_key", eventKey, "kind", kind, "user_id", userID,
			"source_kind", sourceKind, "source_id", sourceID, "provider_gated", gateKey != "")
	}
	return true, nil
}

func (s *Store) releaseNotificationGateTx(ctx context.Context, tx *sql.Tx, gateKey string, now time.Time) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT event_key,payload_json FROM user_notification_events
		WHERE gate_key=? AND queued_at IS NULL ORDER BY created_at,event_key`, gateKey)
	if err != nil {
		return 0, fmt.Errorf("list gated notifications: %w", err)
	}
	type pending struct{ eventKey, payload string }
	items := make([]pending, 0)
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.eventKey, &item.payload); err != nil {
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
	for _, item := range items {
		payload, err := notificationPayloadForRelease(item.payload, now)
		if err != nil {
			return 0, err
		}
		if err := insertOutboxTx(ctx, tx, jobpayload.UserNotificationKind, payload, now, now); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE user_notification_events SET queued_at=?
			WHERE event_key=? AND queued_at IS NULL`, stamp(now), item.eventKey); err != nil {
			return 0, err
		}
	}
	if len(items) > 0 && s.logger != nil {
		s.logger.Info("user notification provider gate released", "gate_key", gateKey, "event_count", len(items))
	}
	return len(items), nil
}

func notificationPayloadForRelease(encoded string, now time.Time) (string, error) {
	var payload jobpayload.UserNotification
	if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
		return "", fmt.Errorf("decode gated notification: %w", err)
	}
	switch payload.Kind {
	case jobpayload.UserEventAutomaticReset, jobpayload.UserEventAddonActivated, jobpayload.UserEventMemberRefundCompleted:
	default:
		return encoded, nil
	}
	completed := now.UTC().Format(time.RFC3339Nano)
	payload.OccurredAt = completed
	payload.Facts[notifications.FactTime] = completed
	return jobpayload.EncodeUserNotification(payload)
}

// ReleaseUserSyncNotifications queues events after an exact user-state sync.
func (s *Store) retryAttempt_ReleaseUserSyncNotifications(ctx context.Context, userID string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := s.releaseNotificationGateTx(ctx, tx, userSyncGate(userID), now); err != nil {
		return err
	}
	return tx.Commit()
}

func providerItemGate(operationID, itemKey string) string {
	return "provider:" + strings.TrimSpace(operationID) + ":" + strings.TrimSpace(itemKey)
}

func userSyncGate(userID string) string { return "user-sync:" + strings.TrimSpace(userID) }

func notificationSource(eventKey string) (string, string, error) {
	parts := strings.SplitN(eventKey, ":", 3)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid user notification event key %q", eventKey)
	}
	return parts[0], parts[1], nil
}
