package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func ensureAbuseRecordTx(ctx context.Context, tx *sql.Tx, userID string, bucket time.Time, qps, limit int, policy abuse.Policy, now time.Time) (string, bool, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT incident_id FROM abuse_incident_facts WHERE user_id=? AND incident_bucket_at=?`, userID, stamp(bucket)).Scan(&id)
	if err == nil {
		return "", false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}
	rules, err := punishmentRulesTx(ctx, tx)
	if err != nil {
		return "", false, err
	}
	var count int
	// Detections suppressed by cooldown must not advance the punishment ladder.
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM abuse_incident_facts fact JOIN abuse_records record ON record.id=fact.incident_id WHERE fact.user_id=? AND fact.created_at>=?`, userID, stamp(now.AddDate(0, 0, -policy.WarningValidityDays))).Scan(&count); err != nil {
		return "", false, err
	}
	action, duration := selectPunishment(rules, count+1)
	id, err = ids.New()
	if err != nil {
		return "", false, err
	}
	blocked, err := abuseRecordCooldownBlockedTx(ctx, tx, userID, policy, now)
	if err != nil {
		return "", false, err
	}
	factAction := action
	if blocked {
		factAction = abuse.Action("none")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO abuse_incident_facts(incident_id,user_id,incident_bucket_at,selected_action,created_at) VALUES(?,?,?,?,?)`, id, userID, stamp(bucket), factAction, stamp(now)); err != nil {
		return "", false, err
	}
	if blocked {
		return "", false, nil
	}
	var expires any
	if action == abuse.ActionTemporaryBan {
		expires = stamp(now.Add(time.Duration(duration) * time.Minute))
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO abuse_records(id,user_id,incident_bucket_at,measured_qps,qps_limit,selected_action,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?)`, id, userID, stamp(bucket), qps, limit, action, expires, stamp(now))
	if err != nil {
		return "", false, fmt.Errorf("insert abuse record: %w", err)
	}
	if action == abuse.ActionTemporaryBan {
		_, err = tx.ExecContext(ctx, `INSERT INTO abuse_temp_bans(record_id,user_id,expires_at,created_at) VALUES(?,?,?,?)`, id, userID, expires, stamp(now))
		if err != nil {
			return "", false, err
		}
	}
	return id, true, nil
}
func punishmentRulesTx(ctx context.Context, tx *sql.Tx) ([]abuse.PunishmentRule, error) {
	rows, err := tx.QueryContext(ctx, `SELECT action,enabled,incident_threshold,duration_minutes,all_nodes,revision FROM abuse_punishment_rules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []abuse.PunishmentRule{}
	for rows.Next() {
		var item abuse.PunishmentRule
		var enabled, all int
		if err = rows.Scan(&item.Action, &enabled, &item.IncidentThreshold, &item.DurationMinutes, &all, &item.Revision); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		item.AllNodes = all == 1
		out = append(out, item)
	}
	return out, rows.Err()
}
func selectPunishment(rules []abuse.PunishmentRule, count int) (abuse.Action, int) {
	order := []abuse.Action{abuse.ActionWarning, abuse.ActionIPBan, abuse.ActionRevoke, abuse.ActionTemporaryBan}
	chosen := abuse.Action("none")
	duration := 0
	for _, action := range order {
		for _, rule := range rules {
			if rule.Action == action && rule.Enabled && count >= rule.IncidentThreshold {
				chosen = action
				duration = rule.DurationMinutes
			}
		}
	}
	return chosen, duration
}
func queueAbuseNotificationsTx(ctx context.Context, tx *sql.Tx, recordID, userID string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT telegram_id FROM users WHERE id=? OR role='admin'`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var telegramID int64
		if err = rows.Scan(&telegramID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_notification_deliveries(record_id,recipient_telegram_id,kind,created_at) VALUES(?,?,?,?)`, recordID, telegramID, "incident", stamp(now))
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed == 1 {
			payload, _ := json.Marshal(map[string]string{"recordId": recordID, "telegramId": strconv.FormatInt(telegramID, 10)})
			if err = insertOutboxTx(ctx, tx, "abuse_notification", string(payload), now, now); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}
func (s *Store) DueTemporaryBans(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM abuse_temp_bans WHERE restored_at IS NULL AND restore_queued_at IS NULL AND expires_at<=?`, stamp(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *Store) QueueRestore(ctx context.Context, userID string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE abuse_temp_bans SET restore_queued_at=? WHERE user_id=? AND restored_at IS NULL AND restore_queued_at IS NULL`, stamp(now), userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return nil
	}
	payload, _ := json.Marshal(map[string]string{"userId": userID})
	if err = insertOutboxTx(ctx, tx, "abuse_restore", string(payload), now, now); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DeleteRecord(ctx context.Context, actorID, recordID string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE abuse_records SET deleted_at=? WHERE id=? AND deleted_at IS NULL`, stamp(now), recordID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM abuse_incident_facts WHERE incident_id=?`, recordID); err != nil {
		return err
	}
	if actorID != "" {
		auditID, idErr := ids.New()
		if idErr != nil {
			return idErr
		}
		if err = insertAuditTx(ctx, tx, auditID, &actorID, "abuse.record.delete", "abuse_record", recordID, "admin profile history delete", now.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
