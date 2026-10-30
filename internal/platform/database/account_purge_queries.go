package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AccountPurge reads only the current durable cleanup generation.
func (s *Store) AccountPurge(ctx context.Context, id string, generation int64) (*model.AccountPurge, error) {
	target := &model.AccountPurge{UserID: id, Generation: generation}
	var cutoff string
	err := s.db.QueryRowContext(ctx, `SELECT last_purged_at FROM users WHERE id=? AND role='user' AND account_status='purging' AND account_generation=?`, id, generation).Scan(&cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if target.Cutoff, err = parseStamp(cutoff); err != nil {
		return nil, err
	}
	var attempted int
	var status string
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(remote_user_id,''),create_attempted,status FROM emby_accounts WHERE user_id=?`, id).Scan(&target.RemoteEmbyID, &attempted, &status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	target.EmbyUncertain = target.RemoteEmbyID == "" && attempted == 1 && status != "failed"
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind<>'account_purge' AND status='processing' AND `+accountJobTargets+`)`, id, id, id, id, id).Scan(&target.Busy)
	return target, err
}

// EnqueueAccountPurges resumes incomplete cleanup after retries or process restarts.
func (s *Store) EnqueueAccountPurges(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,account_generation FROM users WHERE account_status='purging'
 AND NOT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind='account_purge' AND status IN ('pending','processing') AND json_extract(payload,'$.userId')=users.id) ORDER BY id LIMIT 64`)
	if err != nil {
		return err
	}
	type item struct {
		id         string
		generation int64
	}
	items := []item{}
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.id, &i.generation); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, i := range items {
		payload, err := json.Marshal(map[string]any{"userId": i.id, "generation": i.generation})
		if err != nil {
			return err
		}
		if err = s.EnqueueOutbox(ctx, "account_purge", string(payload), now); err != nil {
			return err
		}
	}
	return nil
}

// resumeAccountAuthentication starts a new lifecycle after an explicit unblock.
// Canonical IDs and archived financial rows remain stable; privileges do not.
func (s *Store) resumeAccountAuthentication(ctx context.Context, telegramID int64, now time.Time) error {
	var status string
	var blocked bool
	err := s.db.QueryRowContext(ctx, `SELECT account_status,bot_blocked FROM users WHERE telegram_id=?`, telegramID).Scan(&status, &blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if blocked || status == "purging" {
		return ErrAccountUnavailable
	}
	if status == "purged" {
		result, err := s.db.ExecContext(ctx, `UPDATE users SET account_status='active',onboarding_state='intro',policy_accepted_at=NULL,accepted_agreement_revision=0,created_at=?,updated_at=? WHERE telegram_id=? AND account_status='purged' AND bot_blocked=0`, stamp(now), stamp(now), telegramID)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return ErrAccountUnavailable
		}
	}
	return nil
}
