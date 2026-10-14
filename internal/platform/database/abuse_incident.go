package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Store) CreateIncident(ctx context.Context, userID string, bucket time.Time, qps, limit int, reasons, nodes []string, policy abuse.Policy, now time.Time) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	created, err := createIncidentTx(ctx, tx, abuse.Incident{UserID: userID, OccurredAt: bucket, MeasuredQPS: qps, QPSLimit: limit, Reasons: reasons, Nodes: nodes}, policy, now)
	if err != nil {
		return false, err
	}
	return created, tx.Commit()
}

func createIncidentTx(ctx context.Context, tx *sql.Tx, incident abuse.Incident, policy abuse.Policy, now time.Time) (bool, error) {
	recordID, created, err := ensureAbuseRecordTx(ctx, tx, incident.UserID, incident.OccurredAt, incident.MeasuredQPS, incident.QPSLimit, policy, now)
	if err != nil || recordID == "" {
		return false, err
	}
	for _, reason := range incident.Reasons {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_record_reasons(record_id,name) VALUES(?,?)`, recordID, reason); err != nil {
			return false, err
		}
	}
	for _, node := range incident.Nodes {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_record_nodes(record_id,node_uuid) VALUES(?,?)`, recordID, node); err != nil {
			return false, err
		}
	}
	if !created {
		return false, nil
	}
	payload, _ := json.Marshal(map[string]string{"recordId": recordID})
	if err = insertOutboxTx(ctx, tx, "abuse_punishment", string(payload), now, now); err != nil {
		return false, err
	}
	if err = queueAbuseNotificationsTx(ctx, tx, recordID, incident.UserID, now); err != nil {
		return false, err
	}
	return true, nil
}
