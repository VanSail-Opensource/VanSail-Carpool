package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const abuseClaimTimeout = time.Hour

func (s *Store) WhitelistedUsers(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT users.id FROM users JOIN abuse_whitelist ON abuse_whitelist.remna_user_id=users.remna_user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func (s *Store) RecoverEventClaims(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE abuse_pending_log_events SET claim_token=NULL,claimed_at=NULL WHERE claim_token IS NOT NULL`)
	return err
}

func (s *Store) ClaimEvents(ctx context.Context, cutoff, now time.Time, limit int) (abuse.EventClaim, error) {
	claim := abuse.EventClaim{}
	if limit < 1 {
		return claim, abuse.ErrInvalid
	}
	token, err := ids.New()
	if err != nil {
		return claim, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return claim, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `UPDATE abuse_pending_log_events SET claim_token=NULL,claimed_at=NULL WHERE claimed_at<?`, stamp(now.Add(-abuseClaimTimeout)))
	if err != nil {
		return claim, err
	}
	var boundarySecond, boundaryUser string
	err = tx.QueryRowContext(ctx, `SELECT event_second,user_id FROM abuse_pending_log_events WHERE claim_token IS NULL AND event_second<=? ORDER BY event_second,user_id,id LIMIT 1 OFFSET ?`, stamp(cutoff), limit-1).Scan(&boundarySecond, &boundaryUser)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `UPDATE abuse_pending_log_events SET claim_token=?,claimed_at=? WHERE claim_token IS NULL AND event_second<=?`, token, stamp(now), stamp(cutoff))
	} else if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE abuse_pending_log_events SET claim_token=?,claimed_at=? WHERE claim_token IS NULL AND event_second<=? AND (event_second<? OR (event_second=? AND user_id<=?))`, token, stamp(now), stamp(cutoff), boundarySecond, boundarySecond, boundaryUser)
	}
	if err != nil {
		return claim, err
	}
	claim.Token = token
	claim.Events, err = claimedEventsTx(ctx, tx, token)
	if err != nil {
		return claim, err
	}
	claim.Legacy, err = legacySamplesTx(ctx, tx, cutoff, limit)
	if err != nil {
		return claim, err
	}
	return claim, tx.Commit()
}

func claimedEventsTx(ctx context.Context, tx *sql.Tx, token string) ([]abuse.LogEvent, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,node_uuid,event_second,domain,fingerprint FROM abuse_pending_log_events WHERE claim_token=? ORDER BY event_second,user_id,id`, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []abuse.LogEvent{}
	for rows.Next() {
		var item abuse.LogEvent
		var at string
		if err = rows.Scan(&item.ID, &item.UserID, &item.NodeUUID, &at, &item.Domain, &item.Fingerprint); err != nil {
			return nil, err
		}
		item.EventSecond, err = parseStamp(at)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ReleaseEventClaim(ctx context.Context, token string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE abuse_pending_log_events SET claim_token=NULL,claimed_at=NULL WHERE claim_token=?`, token)
	return err
}
