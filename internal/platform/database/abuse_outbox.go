package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

type AbuseJob struct {
	RecordID, UserID, RemoteUserID, Reason string
	Action                                 abuse.Action
	DurationMinutes                        int
	AllNodes                               bool
	Nodes                                  []string
	QPS, Limit                             int
	ExpiresAt                              *time.Time
}
type AbuseDelivery struct {
	AbuseJob
	TelegramID int64
	Delivered  bool
	Username   string
	Locale     string
	OccurredAt time.Time
}

func (s *Store) AbuseJob(ctx context.Context, recordID string) (AbuseJob, error) {
	var job AbuseJob
	var expires sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT record.id,record.user_id,user.remna_user_id,COALESCE(GROUP_CONCAT(reason.name,', '),'global'),record.selected_action,record.measured_qps,record.qps_limit,record.expires_at FROM abuse_records record JOIN users user ON user.id=record.user_id LEFT JOIN abuse_record_reasons reason ON reason.record_id=record.id WHERE record.id=? GROUP BY record.id`, recordID).Scan(&job.RecordID, &job.UserID, &job.RemoteUserID, &job.Reason, &job.Action, &job.QPS, &job.Limit, &expires)
	if err == sql.ErrNoRows {
		return job, ErrNotFound
	}
	if err != nil {
		return job, err
	}
	if expires.Valid {
		parsed, parseErr := parseStamp(expires.String)
		if parseErr != nil {
			return job, parseErr
		}
		job.ExpiresAt = &parsed
	}
	for _, rule := range mustPunishments(s, ctx) {
		if rule.Action == job.Action {
			job.DurationMinutes = rule.DurationMinutes
			job.AllNodes = rule.AllNodes
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT node_uuid FROM abuse_record_nodes WHERE record_id=?`, recordID)
	if err != nil {
		return job, err
	}
	defer rows.Close()
	for rows.Next() {
		var node string
		if err = rows.Scan(&node); err != nil {
			return job, err
		}
		job.Nodes = append(job.Nodes, node)
	}
	return job, rows.Err()
}
func (s *Store) AbuseDelivery(ctx context.Context, recordID string, telegramID int64) (AbuseDelivery, error) {
	var item AbuseDelivery
	job, err := s.AbuseJob(ctx, recordID)
	if err != nil {
		return item, err
	}
	item.AbuseJob = job
	item.TelegramID = telegramID
	var delivered sql.NullString
	var occurred string
	err = s.db.QueryRowContext(ctx, `SELECT delivery.delivered_at,COALESCE(user.username,''),COALESCE(recipient.notification_locale,'en'),record.incident_bucket_at
		FROM abuse_notification_deliveries delivery
		JOIN abuse_records record ON record.id=delivery.record_id
		JOIN users user ON user.id=record.user_id
		LEFT JOIN users recipient ON recipient.telegram_id=delivery.recipient_telegram_id
		WHERE delivery.record_id=? AND delivery.recipient_telegram_id=? AND delivery.kind='incident'`, recordID, telegramID).
		Scan(&delivered, &item.Username, &item.Locale, &occurred)
	if err != nil {
		return item, err
	}
	item.OccurredAt, err = parseStamp(occurred)
	if err != nil {
		return item, err
	}
	item.Delivered = delivered.Valid
	return item, nil
}
func (s *Store) MarkAbuseDelivery(ctx context.Context, recordID string, telegramID int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE abuse_notification_deliveries SET delivered_at=? WHERE record_id=? AND recipient_telegram_id=? AND kind='incident' AND delivered_at IS NULL`, stamp(now), recordID, telegramID)
	return err
}
func (s *Store) MarkPunishmentCompleted(ctx context.Context, recordID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE abuse_records SET punishment_completed_at=COALESCE(punishment_completed_at,?) WHERE id=?`, stamp(now), recordID)
	return err
}
func (s *Store) AbuseRestoreRemoteID(ctx context.Context, userID string) (string, error) {
	var remote string
	err := s.db.QueryRowContext(ctx, `SELECT user.remna_user_id FROM abuse_temp_bans ban JOIN users user ON user.id=ban.user_id WHERE ban.user_id=? AND ban.restore_queued_at IS NOT NULL AND ban.restored_at IS NULL`, userID).Scan(&remote)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return remote, err
}
func (s *Store) CompleteAbuseRestore(ctx context.Context, userID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE abuse_temp_bans SET restored_at=? WHERE user_id=? AND restored_at IS NULL`, stamp(now), userID)
	return err
}
func (s *Store) PruneAbuseRecordsTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	var err error
	var retentionDays int
	if err = tx.QueryRowContext(ctx, `SELECT warning_validity_days FROM abuse_policy WHERE id=1`).Scan(&retentionDays); err != nil {
		return err
	}
	retentionCutoff := stamp(now.AddDate(0, 0, -retentionDays))
	if counts["abuse_pending_events"], err = deleteCount(ctx, tx, `DELETE FROM abuse_pending_log_events WHERE event_second<? AND claim_token IS NULL`, retentionCutoff); err != nil {
		return err
	}
	if counts["abuse_legacy_samples"], err = deleteCount(ctx, tx, `DELETE FROM abuse_qps_samples WHERE bucket_at<? OR EXISTS (SELECT 1 FROM abuse_detector_state state WHERE state.user_id=abuse_qps_samples.user_id AND state.reason_name=abuse_qps_samples.reason_name AND state.last_bucket_at>=abuse_qps_samples.bucket_at)`, retentionCutoff); err != nil {
		return err
	}
	if counts["abuse_fingerprints"], err = deleteCount(ctx, tx, `DELETE FROM abuse_log_fingerprints WHERE observed_at<?`, stamp(now.Add(-24*time.Hour))); err != nil {
		return err
	}
	if counts["abuse_qps_rollups"], err = deleteCount(ctx, tx, `DELETE FROM abuse_qps_rollups WHERE window_at<?`, stamp(now.Add(-24*time.Hour))); err != nil {
		return err
	}
	if counts["abuse_detector_state"], err = deleteCount(ctx, tx, `DELETE FROM abuse_detector_state WHERE last_bucket_at<?`, stamp(now.Add(-24*time.Hour))); err != nil {
		return err
	}
	if counts["abuse_records"], err = deleteCount(ctx, tx, `DELETE FROM abuse_records WHERE incident_bucket_at<? AND punishment_completed_at IS NOT NULL AND NOT EXISTS (SELECT 1 FROM abuse_notification_deliveries delivery WHERE delivery.record_id=abuse_records.id AND delivery.delivered_at IS NULL) AND NOT EXISTS (SELECT 1 FROM abuse_temp_bans ban WHERE ban.record_id=abuse_records.id AND ban.restored_at IS NULL)`, retentionCutoff); err != nil {
		return err
	}
	if counts["abuse_incident_facts"], err = deleteCount(ctx, tx, `DELETE FROM abuse_incident_facts WHERE incident_bucket_at<?`, retentionCutoff); err != nil {
		return err
	}
	return nil
}
