package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func abuseRecordCooldownBlockedTx(ctx context.Context, tx *sql.Tx, userID string, policy abuse.Policy, now time.Time) (bool, error) {
	if policy.WarningCooldownMinutes == 0 {
		return false, nil
	}
	cutoff := now.UTC().Add(-time.Duration(policy.WarningCooldownMinutes) * time.Minute)
	// Suppressed facts prevent replay, but only emitted records start a cooldown.
	// Include the entire boundary second: RFC3339Nano text has variable precision.
	rows, err := tx.QueryContext(ctx, `SELECT fact.created_at FROM abuse_incident_facts fact JOIN abuse_records record ON record.id=fact.incident_id WHERE fact.user_id=? AND fact.created_at>=? ORDER BY fact.created_at DESC`, userID, cutoff.Format("2006-01-02T15:04:05"))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return false, err
		}
		createdAt, parseErr := parseStamp(raw)
		if parseErr != nil {
			return false, parseErr
		}
		if createdAt.After(cutoff) {
			return true, nil
		}
	}
	return false, rows.Err()
}
